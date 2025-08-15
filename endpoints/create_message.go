package endpoints

import (
	"encoding/json"
	"io"
	"log"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jiaobendaye/go-claude-code-proxy/conversion"
	"github.com/jiaobendaye/go-claude-code-proxy/core"
	"github.com/jiaobendaye/go-claude-code-proxy/models"
)

func CreateMessage(c *gin.Context) {
	var claudeRequest models.ClaudeMessagesRequest
	if err := c.ShouldBindJSON(&claudeRequest); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid JSON format"})
		return
	}

	// Convert Claude request to OpenAI format
	openaiReq := conversion.ConvertClaudeToOpenai(&claudeRequest, core.GetModelManager())
	ctx := c.Request.Context()

	if !claudeRequest.Stream {
		openAiResp, err := openaiClient.CreateChatCompletion(
			ctx,
			*openaiReq,
		)
		if err == nil {
			claudeResp := conversion.ConvertOpeenaiToClaudeResponse(openAiResp, claudeRequest)
			c.JSON(http.StatusOK, claudeResp)
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{"type": "error", "error": gin.H{"type": "api_error", "message": err.Error()}})
		}
	} else {
		stream, err := openaiClient.CreateChatCompletionStream(
			ctx,
			*openaiReq,
		)

		if err != nil {
			log.Printf("Error creating stream: %v\n", err)
			c.JSON(http.StatusInternalServerError, gin.H{"type": "error", "error": gin.H{"type": "api_error", "message": err.Error()}})
			return
		}
		defer stream.Close()
		c.Writer.Header().Set("Content-Type", "text/event-stream")
		c.Writer.Header().Set("Cache-Control", "no-cache")
		c.Writer.Header().Set("Connection", "keep-alive")
		c.Writer.Header().Set("Access-Control-Allow-Origin", "*")
		c.Writer.Header().Set("Access-Control-Allow-Headers", "*")

		// Send initial SSE events
		messageId := "msg_" + strings.ReplaceAll(uuid.New().String(), "-", "")
		c.Writer.WriteString("event: " + core.EVENT_MESSAGE_START + "\ndata: ")
		data, _ := json.Marshal(map[string]any{
			"type": core.EVENT_MESSAGE_START,
			"message": map[string]any{
				"id":            messageId,
				"type":          "message",
				"role":          core.ROLE_ASSISTANT,
				"model":         claudeRequest.Model,
				"content":       []int{},
				"stop_reason":   nil,
				"stop_sequence": nil,
				"usage": map[string]int{
					"input_tokens":  0,
					"output_tokens": 0,
				},
			},
		})
		c.Writer.Write(data)
		c.Writer.WriteString("\n\n")
		c.Writer.Flush()

		c.Writer.WriteString("event: " + core.EVENT_CONTENT_BLOCK_START + "\ndata: ")
		data, _ = json.Marshal(map[string]any{
			"type":  core.EVENT_CONTENT_BLOCK_START,
			"index": 0,
			"content_block": map[string]string{
				"type": core.CONTENT_TEXT,
				"text": "",
			},
		})
		c.Writer.Write(data)
		c.Writer.WriteString("\n\n")
		c.Writer.Flush()

		c.Writer.WriteString("event: " + core.EVENT_PING + "\ndata: ")
		data, _ = json.Marshal(map[string]any{
			"type": core.EVENT_PING,
		})
		c.Writer.Write(data)
		c.Writer.WriteString("\n\n")
		c.Writer.Flush()

		// Process streaming chunks
		textBlockIndex := 0
		toolBlockIndex := 0
		currentToolCalls := make(map[int]map[string]any)
		finalStopReason := core.STOP_END_TURN
		usageData := map[string]int{
			"input_tokens":  0,
			"output_tokens": 0,
		}

	forloop:
		for {
			response, err := stream.Recv()
			select {
			case <-ctx.Done():
				log.Printf("Client disconnected, stopping stream processing %v", messageId)
				c.Writer.WriteString("event: error\ndata: ")
				errorEvent := map[string]interface{}{
					"type": "error",
					"error": map[string]string{
						"type":    "cancelled",
						"message": "Request was cancelled by client",
					},
				}
				errorEventJSON, _ := json.Marshal(errorEvent)
				c.Writer.Write(errorEventJSON)
				c.Writer.WriteString("\n\n")
				c.Writer.Flush()
				return
			default:
			}
			if err != nil {
				if err == io.EOF {
					break forloop
				} else {
					c.Writer.WriteString("event: error\ndata: ")
					errorEvent := map[string]interface{}{
						"type": "error",
						"error": map[string]interface{}{
							"type":    "streaming_error",
							"message": err.Error(),
						},
					}
					errorEventJSON, _ := json.Marshal(errorEvent)
					c.Writer.Write(errorEventJSON)
					c.Writer.WriteString("\n\n")
					c.Writer.Flush()
				}
				log.Printf("Error receiving stream: %v\n", err)
				return
			}

			// Convert Usage data from OpenAI response to Claude format
			if response.Usage != nil {
				usageData["input_tokens"] = response.Usage.PromptTokens
				usageData["output_tokens"] = response.Usage.CompletionTokens
				if response.Usage.PromptTokensDetails != nil {
					usageData["cache_read_input_tokens"] = response.Usage.PromptTokensDetails.CachedTokens
				}
			}

			// Convert OpenAI streaming response to Claude streaming format.
			if len(response.Choices) > 0 {
				choice := response.Choices[0]
				// Handle text delta
				if choice.Delta.Content != "" {
					c.Writer.WriteString("event: " + core.EVENT_CONTENT_BLOCK_DELTA + "\ndata: ")
					deltaData := map[string]interface{}{
						"type":  core.EVENT_CONTENT_BLOCK_DELTA,
						"index": textBlockIndex,
						"delta": map[string]string{
							"type": core.DELTA_TEXT,
							"text": choice.Delta.Content,
						},
					}
					jsonDelta, _ := json.Marshal(deltaData)
					c.Writer.Write(jsonDelta)
					c.Writer.WriteString("\n\n")
					c.Writer.Flush()
				}

				// Handle tool call deltas with improved incremental processing
				for _, toolCall := range choice.Delta.ToolCalls {
					toolCallIndex := 0
					if toolCall.Index != nil {
						toolCallIndex = *toolCall.Index
					}
					if _, exists := currentToolCalls[toolCallIndex]; !exists {
						currentToolCalls[toolCallIndex] = map[string]interface{}{
							"id":           nil,
							"name":         nil,
							"args_buffer":  "",
							"json_sent":    false,
							"claude_index": nil,
							"started":      false,
						}
					}
					toolCallEntry := currentToolCalls[toolCallIndex]

					// Update tool call ID if provided
					if toolCall.ID != "" {
						toolCallEntry["id"] = toolCall.ID
					}

					// Update function name and start content block if we have both id and name
					if toolCall.Function.Name != "" {
						toolCallEntry["name"] = toolCall.Function.Name
					}

					// Start content block when we have complete initial data
					if toolCallEntry["id"] != nil && toolCallEntry["name"] != nil && !toolCallEntry["started"].(bool) {
						toolBlockIndex++
						toolCallEntry["claude_index"] = textBlockIndex + toolBlockIndex
						toolCallEntry["started"] = true
						c.Writer.WriteString("event: " + core.EVENT_CONTENT_BLOCK_START + "\ndata: ")
						contentBlockStart := map[string]interface{}{
							"type":  core.EVENT_CONTENT_BLOCK_START,
							"index": toolCallEntry["claude_index"],
							"content_block": map[string]interface{}{
								"type":  core.CONTENT_TOOL_USE,
								"id":    toolCallEntry["id"],
								"name":  toolCallEntry["name"],
								"input": map[string]interface{}{},
							},
						}
						contentBlockJSON, _ := json.Marshal(contentBlockStart)
						c.Writer.Write(contentBlockJSON)
						c.Writer.WriteString("\n\n")
						c.Writer.Flush()
					}

					// Handle function arguments
					if toolCall.Function.Arguments != "" && toolCallEntry["started"].(bool) {
						toolCallEntry["args_buffer"] = toolCallEntry["args_buffer"].(string) + toolCall.Function.Arguments
						var parsedArgs map[string]interface{}
						if json.Unmarshal([]byte(toolCallEntry["args_buffer"].(string)), &parsedArgs) == nil {
							if !toolCallEntry["json_sent"].(bool) {
								c.Writer.WriteString("event: " + core.EVENT_CONTENT_BLOCK_DELTA + "\ndata: ")
								contentBlockDelta := map[string]interface{}{
									"type":  core.EVENT_CONTENT_BLOCK_DELTA,
									"index": toolCallEntry["claude_index"],
									"delta": map[string]string{
										"type":         core.DELTA_INPUT_JSON,
										"partial_json": toolCallEntry["args_buffer"].(string),
									},
								}
								contentDeltaJSON, _ := json.Marshal(contentBlockDelta)
								c.Writer.Write(contentDeltaJSON)
								c.Writer.WriteString("\n\n")
								c.Writer.Flush()
								toolCallEntry["json_sent"] = true
							}
						}
					}
				}

				// Handle finish reason
				switch choice.FinishReason {
				case "length":
					finalStopReason = core.STOP_MAX_TOKENS
				case "tool_calls", "function_call":
					finalStopReason = core.STOP_TOOL_USE
				default:
					finalStopReason = core.STOP_END_TURN
				}
				if choice.FinishReason != "" {
					break forloop
				}
			}
		}

		// Send final SSE events
		for _, toolData := range currentToolCalls {
			if toolDataStarted, ok := toolData["started"].(bool); ok && toolDataStarted {
				claudeIndex, _ := toolData["claude_index"].(int)
				c.Writer.WriteString("event: " + core.EVENT_CONTENT_BLOCK_STOP + "\ndata: ")
				contentBlockStop := map[string]interface{}{
					"type":  core.EVENT_CONTENT_BLOCK_STOP,
					"index": claudeIndex,
				}
				contentBlockStopJSON, _ := json.Marshal(contentBlockStop)
				c.Writer.Write(contentBlockStopJSON)
				c.Writer.WriteString("\n\n")
				c.Writer.Flush()
			}
		}

		c.Writer.WriteString("event: " + core.EVENT_MESSAGE_DELTA + "\ndata: ")
		messageDelta := map[string]interface{}{
			"type": core.EVENT_MESSAGE_DELTA,
			"delta": map[string]interface{}{
				"stop_reason":   finalStopReason,
				"stop_sequence": nil,
				"usage":         usageData,
			},
		}
		messageDeltaJSON, _ := json.Marshal(messageDelta)
		c.Writer.Write(messageDeltaJSON)
		c.Writer.WriteString("\n\n")
		c.Writer.Flush()

		c.Writer.WriteString("event: " + core.EVENT_MESSAGE_STOP + "\ndata: ")
		messageStop := map[string]string{
			"type": core.EVENT_MESSAGE_STOP,
		}
		messageStopJSON, _ := json.Marshal(messageStop)
		c.Writer.Write(messageStopJSON)
		c.Writer.WriteString("\n\n")
		c.Writer.Flush()
	}
}
