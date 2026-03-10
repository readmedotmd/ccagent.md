package claudecode

import "io"

// Options configures the Claude Agent SDK behavior.
type Options struct {
	AllowedTools         []string
	DisallowedTools      []string
	SystemPrompt         *string
	AppendSystemPrompt   *string
	Model                *string
	MaxThinkingTokens    int
	PermissionMode       *PermissionMode
	ContinueConversation bool
	Resume               *string
	MaxTurns             int
	Cwd                  *string
	McpServers           map[string]McpServerConfig
	Agents               map[string]AgentDefinition
	ExtraArgs            map[string]*string
	ExtraEnv             map[string]string
	CLIPath              *string
	DebugWriter          io.Writer
	StderrCallback       func(string)
	SettingSources       []string
}

// Option configures Options using the functional options pattern.
type Option func(*Options)

// NewOptions creates Options with default values and applies functional options.
func NewOptions(opts ...Option) *Options {
	o := &Options{
		MaxThinkingTokens: 8000,
		McpServers:        make(map[string]McpServerConfig),
		ExtraArgs:         make(map[string]*string),
		ExtraEnv:          make(map[string]string),
	}
	for _, opt := range opts {
		opt(o)
	}
	return o
}

// WithAllowedTools sets the tools the agent is allowed to use.
func WithAllowedTools(tools ...string) Option {
	return func(o *Options) { o.AllowedTools = tools }
}

// WithDisallowedTools sets the tools the agent is not allowed to use.
func WithDisallowedTools(tools ...string) Option {
	return func(o *Options) { o.DisallowedTools = tools }
}

// WithSystemPrompt sets the system prompt for the conversation.
func WithSystemPrompt(prompt string) Option {
	return func(o *Options) { o.SystemPrompt = &prompt }
}

// WithAppendSystemPrompt sets text to append to the default system prompt.
func WithAppendSystemPrompt(prompt string) Option {
	return func(o *Options) { o.AppendSystemPrompt = &prompt }
}

// WithModel sets the model to use for the conversation.
func WithModel(model string) Option {
	return func(o *Options) { o.Model = &model }
}

// WithMaxThinkingTokens sets the maximum number of thinking tokens.
func WithMaxThinkingTokens(tokens int) Option {
	return func(o *Options) { o.MaxThinkingTokens = tokens }
}

// WithPermissionMode sets the permission handling mode.
func WithPermissionMode(mode PermissionMode) Option {
	return func(o *Options) { o.PermissionMode = &mode }
}

// WithContinueConversation sets whether to continue an existing conversation.
func WithContinueConversation(v bool) Option {
	return func(o *Options) { o.ContinueConversation = v }
}

// WithResume sets the session ID to resume.
func WithResume(sessionID string) Option {
	return func(o *Options) { o.Resume = &sessionID }
}

// WithCwd sets the working directory for the CLI process.
func WithCwd(cwd string) Option {
	return func(o *Options) { o.Cwd = &cwd }
}

// WithMcpServers sets the MCP server configurations.
func WithMcpServers(servers map[string]McpServerConfig) Option {
	return func(o *Options) { o.McpServers = servers }
}

// WithAgents sets the sub-agent definitions.
func WithAgents(agents map[string]AgentDefinition) Option {
	return func(o *Options) { o.Agents = agents }
}

// WithCLIPath sets a custom path to the Claude CLI binary.
func WithCLIPath(path string) Option {
	return func(o *Options) { o.CLIPath = &path }
}
