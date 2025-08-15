package models

type ClaudeContentBlockText struct {
    Type string `json:"type"`
    Text string `json:"text"`
}

type ClaudeContentBlockImage struct {
    Type   string         `json:"type"`
    Source map[string]any `json:"source"`
}

type ClaudeContentBlockToolUse struct {
    Type  string         `json:"type"`
    ID    string         `json:"id"`
    Name  string         `json:"name"`
    Input map[string]any `json:"input"`
}

type ClaudeContentBlockToolResult struct {
    Type       string         `json:"type"`
    ToolUseID  string         `json:"tool_use_id"`
    Content    any            `json:"content"`
}

type ClaudeSystemContent struct {
    Type string `json:"type"`
    Text string `json:"text"`
}

type ClaudeMessage struct {
    Role    string       `json:"role"`
    Content any          `json:"content"`
}

type ClaudeTool struct {
    Name        string         `json:"name"`
    Description string         `json:"description,omitempty"`
    InputSchema map[string]any `json:"input_schema"`
}

type ClaudeThinkingConfig struct {
    Enabled bool `json:"enabled"`
}

type ClaudeMessagesRequest struct {
    Model         string               `json:"model"`
    MaxTokens     int                  `json:"max_tokens"`
    Messages      []ClaudeMessage      `json:"messages"`
    System        any                  `json:"system,omitempty"`
    StopSequences []string             `json:"stop_sequences,omitempty"`
    Stream        bool                 `json:"stream,omitempty"`
    Temperature   float32              `json:"temperature,omitempty"`
    TopP          float32              `json:"top_p,omitempty"`
    TopK          int                  `json:"top_k,omitempty"`
    Metadata      map[string]any       `json:"metadata,omitempty"`
    Tools         []ClaudeTool         `json:"tools,omitempty"`
    ToolChoice    map[string]any       `json:"tool_choice,omitempty"`
    Thinking      ClaudeThinkingConfig `json:"thinking,omitempty"`
}

type ClaudeTokenCountRequest struct {
    Model      string               `json:"model"`
    Messages   []ClaudeMessage      `json:"messages"`
    System     any                  `json:"system,omitempty"`
    Tools      []ClaudeTool         `json:"tools,omitempty"`
    Thinking   ClaudeThinkingConfig `json:"thinking,omitempty"`
    ToolChoice map[string]any       `json:"tool_choice,omitempty"`
}