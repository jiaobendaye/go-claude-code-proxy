package conversion

import (
	"encoding/json"
	"fmt"
	"math"
	"strings"

	"github.com/jiaobendaye/go-claude-code-proxy/core"
	"github.com/jiaobendaye/go-claude-code-proxy/models"
	"github.com/sashabaranov/go-openai"
)

func ConvertClaudeToOpenai(claudeRequest *models.ClaudeMessagesRequest, modelManager *core.ModelManager) *openai.ChatCompletionRequest {
	convertedMessages := []openai.ChatCompletionMessage{}

	// Add system message if present
	if claudeRequest.System != nil {
		var systemText string
		if text, ok := claudeRequest.System.(string); ok {
			systemText = text
		} else if blocks, ok := claudeRequest.System.([]interface{}); ok {
			textParts := []string{}
			for _, block := range blocks {
				if textBlock, blockOk := block.(models.ClaudeContentBlockText); blockOk {
					textParts = append(textParts, textBlock.Text)
				} else if blockMap, blockMapOk := block.(map[string]interface{}); blockMapOk {
					if text, ok := blockMap["text"].(string); blockMap["type"] == core.CONTENT_TEXT && ok {
						textParts = append(textParts, text)
					}
				}
			}
			systemText = strings.Join(textParts, "\n\n")
		}
		if strings.TrimSpace(systemText) != "" {
			convertedMessages = append(convertedMessages, openai.ChatCompletionMessage{
				Role:    core.ROLE_SYSTEM,
				Content: strings.TrimSpace(systemText),
			})
		}
	}

	for i, msg := range claudeRequest.Messages {
		if msg.Role == core.ROLE_USER {
			convertedMessages = append(convertedMessages, *convertClaudeUserMessage(msg))
		} else if msg.Role == core.ROLE_ASSISTANT {
			convertedMessages = append(convertedMessages, *convertClaudeAssistantMessage(msg))

			// Check if next message contains tool results
			if i+1 < len(claudeRequest.Messages) {
				nextMsg := claudeRequest.Messages[i+1]
				if nextMsg.Role == core.ROLE_USER {
					if _, ok := nextMsg.Content.([]any); ok {
						hasToolResult := false
						for _, block := range nextMsg.Content.([]any) {
							if val, ok := core.GetField(block, "type"); ok && val == core.CONTENT_TOOL_RESULT {
								hasToolResult = true
								break
							}
						}
						if hasToolResult {
							// Process tool results
							toolResultMessages := convertClaudeToolResultMessage(nextMsg)
							convertedMessages = append(convertedMessages, toolResultMessages...)
							i++ // Skip processing the tool result message in the next iteration
						}
					}
				}
			}
		}
	}

	// Convert tools
	var openaiTools []openai.Tool
	if claudeRequest.Tools != nil {
		for _, tool := range claudeRequest.Tools {
			if tool.Name != "" {
				openaiTools = append(openaiTools, openai.Tool{
					Type: core.TOOL_FUNCTION,
					Function: &openai.FunctionDefinition{
						Name:        tool.Name,
						Description: tool.Description,
						Parameters:  tool.InputSchema,
					},
				})
			}
		}
	}

	// Convert tool choice
	var toolChoice any
	if claudeRequest.ToolChoice != nil {
		if typeVal, ok := claudeRequest.ToolChoice["type"].(string); ok {
			if typeVal == "tool" {
				nameVal, nameExists := claudeRequest.ToolChoice["name"].(string)
				if nameExists && nameVal != "" {
					toolChoice = openai.ToolChoice{
						Type: core.TOOL_FUNCTION,
						Function: openai.ToolFunction{
							Name: nameVal,
						},
					}
				}
			}
		}

		if toolChoice == nil {
			toolChoice = "auto"
		}
	}

	config := core.GetConfig()
	openaiRequest := &openai.ChatCompletionRequest{
		Model:       modelManager.MapClaudeModelToOpenAI(claudeRequest.Model),
		MaxTokens:   int(math.Min(math.Max(float64(claudeRequest.MaxTokens), float64(config.MinTokensLimit)), float64(config.MaxTokensLimit))),
		Messages:    convertedMessages,
		Stop:        claudeRequest.StopSequences,
		Stream:      claudeRequest.Stream,
		Temperature: claudeRequest.Temperature,
		TopP:        claudeRequest.TopP,
		Tools:       openaiTools,
		ToolChoice:  toolChoice,
	}

	if claudeRequest.Stream {
		openaiRequest.StreamOptions = &openai.StreamOptions{
			IncludeUsage: true,
		}
	}

	return openaiRequest
}

func convertClaudeUserMessage(msg models.ClaudeMessage) *openai.ChatCompletionMessage {
	ret := &openai.ChatCompletionMessage{Role: core.ROLE_USER}
	if msg.Content == nil {
		return ret
	}

	if text, ok := msg.Content.(string); ok {
		ret.Content = text
		return ret
	}

	// Handle multimodal content
	openaiContent := []map[string]string{}
	if blocks, ok := msg.Content.([]any); ok {
		for _, block := range blocks {
			// 将 block 断言为 map[string]any
			if blockMap, ok := block.(map[string]any); ok {
				// 处理文本块
				if blockType, ok := blockMap["type"].(string); ok && blockType == "text" {
					if text, ok := blockMap["text"].(string); ok {
						openaiContent = append(openaiContent, map[string]string{"type": "text", "text": text})
					}
				} else if blockType, ok := blockMap["type"].(string); ok && blockType == "image" { // 处理图片块
					if source, ok := blockMap["source"].(map[string]any); ok {
						if sourceType, ok := source["type"].(string); ok && sourceType == "base64" {
							if mediaType, ok := source["media_type"].(string); ok {
								if data, ok := source["data"].(string); ok {
									s, _ := json.Marshal(map[string]string{"url": "data:" + mediaType + ";base64," + data})
									openaiContent = append(openaiContent, map[string]string{
										"type":      "image_url",
										"image_url": string(s),
									})
								}
							}
						}
					}
				}
			}
		}
	}

	// Simplify content if there's only one text block
	if len(openaiContent) == 1 && openaiContent[0]["type"] == "text" {
		ret.Content = openaiContent[0]["text"]
	} else {
		// Serialize multimodal content into a JSON-like string
		contentBytes, err := json.Marshal(openaiContent)
		if err != nil {
			// ret.Content = "Error serializing content"
		} else {
			ret.Content = string(contentBytes)
		}
	}

	return ret
}

func convertClaudeAssistantMessage(msg models.ClaudeMessage) *openai.ChatCompletionMessage {
	textParts := []string{}
	toolCalls := []openai.ToolCall{}
	ret := &openai.ChatCompletionMessage{
		Role: core.ROLE_ASSISTANT,
	}
	if msg.Content == nil {
		return ret
	}

	// if msg.content is a string, convert it to text block
	if text, ok := msg.Content.(string); ok {
		ret.Content = text
		return ret
	}

	if blocks, ok := msg.Content.([]any); ok {
		for _, block := range blocks {
			if text, ok := block.(models.ClaudeContentBlockText); ok {
				textParts = append(textParts, text.Text)
			} else if tool, ok := block.(models.ClaudeContentBlockToolUse); ok {
				strInput, err := json.Marshal(tool.Input)
				if err != nil {
					fmt.Printf("Error marshalling tool input: %v\n", err)
					continue
				}
				toolCalls = append(toolCalls, openai.ToolCall{
					ID:   tool.ID,
					Type: openai.ToolType(core.TOOL_FUNCTION),
					Function: openai.FunctionCall{
						Name:      tool.Name,
						Arguments: string(strInput),
					},
				})
			}
		}
	}

	if len(textParts) > 0 {
		ret.Content = strings.Join(textParts, "")
	}
	if len(toolCalls) > 0 {
		ret.ToolCalls = toolCalls
	}

	return ret
}

func convertClaudeToolResultMessage(msg models.ClaudeMessage) []openai.ChatCompletionMessage {
	if msg.Content == nil {
		return []openai.ChatCompletionMessage{}
	}

	parsedMessages := []openai.ChatCompletionMessage{}
	if blocks, ok := msg.Content.([]any); ok {
		for _, block := range blocks {
			if toolResult, ok := block.(models.ClaudeContentBlockToolResult); ok {
				parsedMessages = append(parsedMessages, openai.ChatCompletionMessage{
					Role:       core.ROLE_TOOL,
					Content:    parseToolResultContent(toolResult.Content),
					ToolCallID: toolResult.ToolUseID,
				})
			}
		}
	}
	return parsedMessages
}

func parseToolResultContent(content any) string {
	if content == nil {
		return "No content provided"
	}

	if text, ok := content.(string); ok {
		return text
	}

	if contentList, ok := content.([]any); ok {
		resultParts := []string{}
		for _, item := range contentList {
			if block, ok := item.(map[string]any); ok {
				// if block["type"] == core.CONTENT_TEXT {
				// 	if text, ok := block["text"].(string); ok {
				// 		resultParts = append(resultParts, text)
				// 	}
				// }
				if text, ok := block["text"].(string); ok {
					resultParts = append(resultParts, text)
				} else if serializedBlock, err := json.Marshal(item); err == nil {
					resultParts = append(resultParts, string(serializedBlock))
				} else {
					resultParts = append(resultParts, fmt.Sprintf("%v", item))
				}
			} else if text, ok := item.(string); ok {
				resultParts = append(resultParts, text)
			}
		}
		return strings.Join(resultParts, "\n")
	}

	if contentMap, ok := content.(map[string]any); ok {
		if contentMap["type"] == core.CONTENT_TEXT {
			if text, ok := contentMap["text"].(string); ok {
				return text
			}
			return ""
		}
	}

	if serialized, err := json.Marshal(content); err == nil {
		return string(serialized)
	}
	return "Unparseable content"
}
