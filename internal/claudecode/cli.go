package claudecode

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

// findCLI searches for the Claude CLI binary in standard locations.
func findCLI() (string, error) {
	if path, err := exec.LookPath("claude"); err == nil {
		return path, nil
	}

	homeDir, _ := os.UserHomeDir()
	if homeDir == "" {
		homeDir = "."
	}

	var locations []string
	if runtime.GOOS == "windows" {
		locations = []string{
			filepath.Join(homeDir, "AppData", "Roaming", "npm", "claude.cmd"),
			filepath.Join("C:", "Program Files", "nodejs", "claude.cmd"),
			filepath.Join(homeDir, ".npm-global", "claude.cmd"),
			filepath.Join(homeDir, "node_modules", ".bin", "claude.cmd"),
		}
	} else {
		locations = []string{
			filepath.Join(homeDir, ".npm-global", "bin", "claude"),
			"/usr/local/bin/claude",
			filepath.Join(homeDir, ".local", "bin", "claude"),
			filepath.Join(homeDir, "node_modules", ".bin", "claude"),
			filepath.Join(homeDir, ".yarn", "bin", "claude"),
			"/opt/homebrew/bin/claude",
			"/usr/local/homebrew/bin/claude",
		}
	}

	for _, location := range locations {
		if info, err := os.Stat(location); err == nil && !info.IsDir() {
			if runtime.GOOS != "windows" && info.Mode()&0o111 == 0 {
				continue
			}
			return location, nil
		}
	}

	if _, err := exec.LookPath("node"); err != nil {
		return "", NewCLINotFoundError("",
			"Claude Code requires Node.js, which is not installed.\n\n"+
				"Install Node.js from: https://nodejs.org/\n\n"+
				"After installing Node.js, install Claude Code:\n"+
				"  npm install -g @anthropic-ai/claude-code")
	}

	return "", NewCLINotFoundError("",
		"Claude Code not found. Install with:\n"+
			"  npm install -g @anthropic-ai/claude-code\n\n"+
			"If already installed locally, try:\n"+
			`  export PATH="$HOME/node_modules/.bin:$PATH"`+"\n\n"+
			"Or specify the path when creating client")
}

// allowedExtraArgs is the set of CLI flags that may be passed via ExtraArgs.
// Only explicitly allowed flags are forwarded to prevent callers from
// accidentally overriding security-sensitive flags (e.g. permission-mode,
// output-format) or breaking the streaming protocol.
var allowedExtraArgs = map[string]bool{
	"mcp-config":       true,
	"max-thinking":     true,
	"budget-tokens":    true,
	"notify":           true,
	"no-user-prompts":  true,
	"prefill":          true,
	"profile":          true,
	"project-dir":      true,
}

// buildCommand constructs the CLI command with all necessary flags.
func buildCommand(cliPath string, options *Options, closeStdin bool) []string {
	cmd := []string{cliPath, "--output-format", "stream-json", "--verbose"}

	if closeStdin {
		cmd = append(cmd, "--print")
	} else {
		cmd = append(cmd, "--input-format", "stream-json")
	}

	if options == nil {
		cmd = append(cmd, "--setting-sources", "")
		return cmd
	}

	if len(options.AllowedTools) > 0 {
		cmd = append(cmd, "--allowed-tools", strings.Join(options.AllowedTools, ","))
	}
	if len(options.DisallowedTools) > 0 {
		cmd = append(cmd, "--disallowed-tools", strings.Join(options.DisallowedTools, ","))
	}
	if options.SystemPrompt != nil {
		cmd = append(cmd, "--system-prompt", *options.SystemPrompt)
	}
	if options.AppendSystemPrompt != nil {
		cmd = append(cmd, "--append-system-prompt", *options.AppendSystemPrompt)
	}
	if options.Model != nil {
		cmd = append(cmd, "--model", *options.Model)
	}
	if options.PermissionMode != nil {
		cmd = append(cmd, "--permission-mode", string(*options.PermissionMode))
	}
	if options.ContinueConversation {
		cmd = append(cmd, "--continue")
	}
	if options.Resume != nil {
		cmd = append(cmd, "--resume", *options.Resume)
	}
	if options.MaxTurns > 0 {
		cmd = append(cmd, "--max-turns", fmt.Sprintf("%d", options.MaxTurns))
	}

	// Agents
	if len(options.Agents) > 0 {
		agentsMap := make(map[string]map[string]any)
		for name, agent := range options.Agents {
			agentMap := map[string]any{
				"description": agent.Description,
				"prompt":      agent.Prompt,
			}
			if len(agent.Tools) > 0 {
				agentMap["tools"] = agent.Tools
			}
			if agent.Model != "" {
				agentMap["model"] = string(agent.Model)
			}
			agentsMap[name] = agentMap
		}
		if data, err := json.Marshal(agentsMap); err == nil {
			cmd = append(cmd, "--agents", string(data))
		}
	}

	// Setting sources (always pass, even if empty)
	sourcesValue := ""
	if len(options.SettingSources) > 0 {
		sourcesValue = strings.Join(options.SettingSources, ",")
	}
	cmd = append(cmd, "--setting-sources", sourcesValue)

	// Extra args (includes --mcp-config when MCP servers are configured).
	// Only explicitly allowed flags are forwarded to the CLI.
	for flag, value := range options.ExtraArgs {
		if !allowedExtraArgs[flag] {
			continue
		}
		if value == nil {
			cmd = append(cmd, "--"+flag)
		} else {
			cmd = append(cmd, "--"+flag, *value)
		}
	}

	return cmd
}

// validateWorkingDirectory checks if the working directory exists, is valid,
// and resolves symlinks to prevent symlink-based path traversal.
func validateWorkingDirectory(cwd string) error {
	if cwd == "" {
		return nil
	}

	// Resolve symlinks to get the real path.
	resolved, err := filepath.EvalSymlinks(cwd)
	if err != nil {
		if os.IsNotExist(err) {
			return NewConnectionError(fmt.Sprintf("working directory does not exist: %s", cwd), err)
		}
		return fmt.Errorf("failed to resolve working directory: %w", err)
	}

	info, err := os.Stat(resolved)
	if err != nil {
		return fmt.Errorf("failed to check working directory: %w", err)
	}
	if !info.IsDir() {
		return NewConnectionError(fmt.Sprintf("working directory path is not a directory: %s", cwd), nil)
	}
	return nil
}
