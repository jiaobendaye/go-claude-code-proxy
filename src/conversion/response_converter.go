package conversion

import (
	"encoding/json"

	"github.com/jiaobendaye/go-claude-code-proxy/src/core"
	"github.com/jiaobendaye/go-claude-code-proxy/src/models"
	"github.com/sashabaranov/go-openai"
)

// Convert OpenAI response to Claude format.
func ConvertOpeenaiToClaudeResponse(openaiResponse openai.ChatCompletionResponse, originalRequest models.ClaudeMessagesRequest) map[string]any {
	choices := openaiResponse.Choices
	if len(choices) == 0 {
		return map[string]any{"error": "No choices in OpenAI response"}
	}

	choice := choices[0]
	message := choice.Message

	contentBlocks := []map[string]any{}

	// Add text content
	if message.Content != "" {
		contentBlocks = append(contentBlocks, map[string]any{
			"type": core.CONTENT_TEXT,
			"text": message.Content,
		})
	}

	// Add tool calls
	for _, toolCall := range message.ToolCalls {
		if toolCall.Type == core.TOOL_FUNCTION {
			arguments := map[string]any{}

			if err := json.Unmarshal([]byte(toolCall.Function.Arguments), &arguments); err != nil {
				arguments["raw_input"] = toolCall.Function.Arguments
			}

			contentBlocks = append(contentBlocks, map[string]any{
				"type":  core.CONTENT_TOOL_USE,
				"id":    toolCall.ID,
				"name":  toolCall.Function.Name,
				"input": arguments,
			})
		}
	}

	// Ensure at least one content block
	if len(contentBlocks) == 0 {
		contentBlocks = append(contentBlocks, map[string]any{
			"type": core.CONTENT_TEXT,
			"text": "",
		})
	}

	// Map finish reason
	stopReason := core.STOP_END_TURN // Default value
	switch choice.FinishReason {
	case "length":
		stopReason = core.STOP_MAX_TOKENS
	case "tool_calls", "function_call":
		stopReason = core.STOP_TOOL_USE
	}

	claudeResponse := map[string]any{
		"id":            openaiResponse.ID,
		"type":          "message",
		"role":          "assistant",
		"model":         originalRequest.Model,
		"content":       contentBlocks,
		"stop_reason":   stopReason,
		"stop_sequence": nil,
		"usage": map[string]any{
			"input_tokens":  openaiResponse.Usage.PromptTokens,
			"output_tokens": openaiResponse.Usage.CompletionTokens,
		},
	}

	return claudeResponse
}
