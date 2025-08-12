package endpoints

import (
	"context"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jiaobendaye/go-claude-code-proxy/src/conversion"
	"github.com/jiaobendaye/go-claude-code-proxy/src/core"
	"github.com/jiaobendaye/go-claude-code-proxy/src/models"
	"github.com/sashabaranov/go-openai"
)

var openaiClient *openai.Client

func initClient() {
	config := core.GetConfig()
	openaiConfig := openai.DefaultConfig(config.OpenAIAPIKey)
	openaiConfig.BaseURL = config.OpenAIBaseURL
	openaiClient = openai.NewClientWithConfig(openaiConfig)
}

func ValidateAPI(c *gin.Context) {
	config := core.GetConfig()

	clientAPIKey := c.GetHeader("x-api-key")
	if clientAPIKey == "" {
		authorization := c.GetHeader("Authorization")
		if authorization != "" && len(authorization) > 7 && authorization[:7] == "Bearer " {
			clientAPIKey = authorization[7:]
		}
	}

	if !config.ValidateClientAPIKey(clientAPIKey) {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid API key. Please provide a valid Anthropic API key."})
		c.Abort()
		return
	}

	c.Next()
}

// Initialize client and the router for gin endpoints and attach middleware for configuration validation.
func SetupOpenaiClientRouter() *gin.Engine {
	initClient()

	router := gin.Default()
	router.Use(ValidateAPI)

	// Define routes
	router.POST("/v1/messages", CreateMessage)
	router.POST("/v1/messages/count_tokens", CountTokens)
	router.GET("/health", HealthCheck)
	router.GET("/test-connection", TestConnection)
	router.GET("/", RootEndpoint)

	return router
}

// Placeholder for CreateMessage endpoint
func CreateMessage(c *gin.Context) {
	var claudeRequest models.ClaudeMessagesRequest
	if err := c.ShouldBindJSON(&claudeRequest); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid JSON format"})
		return
	}

	//  Convert Claude request to OpenAI format
	openaiReq := conversion.ConvertClaudeToOpenai(&claudeRequest, core.GetModelManager())
	ctx := c.Request.Context()

	if !claudeRequest.Stream {
		opeanAiResp, err := openaiClient.CreateChatCompletion(
			ctx,
			*openaiReq,
		)
		if err == nil {
			claudeResp := conversion.ConvertOpeenaiToClaudeResponse(opeanAiResp, claudeRequest)
			c.JSON(http.StatusOK, claudeResp)
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{"type": "error", "error": gin.H{"type": "api_error", "message": err.Error()}})
		}
	} else {
		// client.CreateCompletionStream()
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
		c.Writer.WriteString("event: " + core.EVENT_MESSAGE_START + "\ndata:")
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

		c.Writer.WriteString("event: " + core.EVENT_CONTENT_BLOCK_START + "\ndata:")
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

		c.Writer.WriteString("event: " + core.EVENT_PING + "\ndata:")
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

	forloop:
		for {
			response, err := stream.Recv()
			select {
			case <-ctx.Done():
				// Handle cancellation
				log.Printf("Client disconnected, stopping stream processing %v", messageId)
				c.Writer.WriteString("event: error\ndata:")
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
				// 如果流结束或出现错误，就停止处理
				if err == io.EOF {
					break forloop
				} else {
					// Handle any streaming errors gracefully
					c.Writer.WriteString("event: error\ndata:")
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

			// Convert OpenAI streaming response to Claude streaming format.
			if len(response.Choices) > 0 {
				choice := response.Choices[0]
				// Handle text delta
				if choice.Delta.Content != "" {
					c.Writer.WriteString("event: " + core.EVENT_CONTENT_BLOCK_DELTA + "\ndata:")
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
					toolCallIndex := *toolCall.Index
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
						c.Writer.WriteString("event: " + core.EVENT_CONTENT_BLOCK_START + "\ndata:")
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
								c.Writer.WriteString("event: " + core.EVENT_CONTENT_BLOCK_DELTA + "\ndata:")
								contentBlockDelta := map[string]interface{}{
									"type":  core.EVENT_CONTENT_BLOCK_DELTA,
									"index": toolCallEntry["claude_index"],
									"delta": map[string]interface{}{
										"type":         core.DELTA_INPUT_JSON,
										"partial_json": parsedArgs,
									},
								}
								contentDeltaJSON, _ := json.Marshal(contentBlockDelta)
								c.Writer.Write(contentDeltaJSON)
								c.Writer.WriteString("\n\n")
								toolCallEntry["json_sent"] = true
								c.Writer.Flush()
							}
						}
					}
				}

				// Handle finish reason
				switch choice.FinishReason {
				case "stop":
					finalStopReason = core.STOP_END_TURN
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
				c.Writer.WriteString("event: " + core.EVENT_CONTENT_BLOCK_STOP + "\ndata:")
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
		c.Writer.WriteString("event: " + core.EVENT_MESSAGE_DELTA + "\ndata:")
		messageDelta := map[string]interface{}{
			"type": core.EVENT_MESSAGE_DELTA,
			"delta": map[string]interface{}{
				"stop_reason":   finalStopReason,
				"stop_sequence": nil,
			},
		}
		messageDeltaJSON, _ := json.Marshal(messageDelta)
		c.Writer.Write(messageDeltaJSON)
		c.Writer.WriteString("\n\n")
		c.Writer.Flush()

		c.Writer.WriteString("event: " + core.EVENT_MESSAGE_STOP + "\ndata:")
		messageStop := map[string]string{
			"type": core.EVENT_MESSAGE_STOP,
		}
		messageStopJSON, _ := json.Marshal(messageStop)
		c.Writer.Write(messageStopJSON)
		c.Writer.WriteString("\n\n")
		c.Writer.Flush()
	}
}

// Placeholder for CountTokens endpoint
func CountTokens(c *gin.Context) {
	var claudeReq models.ClaudeMessagesRequest
	err := c.ShouldBindJSON(&claudeReq)
	if err != nil {
		log.Panicf("Error binding JSON: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request format"})
		return
	}

	totalChars := 0
	// Count system message characters
	if claudeReq.System != nil {
		if sysStr, ok := claudeReq.System.(string); ok {
			totalChars += len(sysStr)
		} else if sysArr, ok := claudeReq.System.([]any); ok {
			for _, block := range sysArr {
				if text, valid := core.GetTextField(block); valid {
					totalChars += len(text)
				}
			}
		}
	}

	// Count message characters
	for _, msg := range claudeReq.Messages {
		if msg.Content != nil {
			if msgStr, ok := msg.Content.(string); ok {
				totalChars += len(msgStr)
			} else if msgArr, ok := msg.Content.([]any); ok {
				for _, block := range msgArr {
					if text, valid := core.GetTextField(block); valid {
						totalChars += len(text)
					}
				}
			}
		}
	}

	// Rough estimation: 4 characters per token
	estimatedTokens := totalChars / 4
	if estimatedTokens == 0 {
		estimatedTokens = 1
	}

	c.JSON(http.StatusOK, gin.H{"input_tokens": estimatedTokens})
}

// Placeholder for HealthCheck endpoint
func HealthCheck(c *gin.Context) {
	config := core.GetConfig()
	c.JSON(http.StatusOK, gin.H{
		"status":                    "healthy",
		"timestamp":                 time.Now().Format(time.RFC3339),
		"openai_api_configured":     config.OpenAIAPIKey != "",
		"api_key_valid":             config.ValidateAPIKey(),
		"client_api_key_validation": config.AnthropicAPIKey != "",
	})
}

// Placeholder for TestConnection endpoint
func TestConnection(c *gin.Context) {
	config := core.GetConfig()

	// Simulate OpenAI API call
	resp, err := openaiClient.CreateChatCompletion(
		context.Background(),
		openai.ChatCompletionRequest{
			Model: config.SmallModel,
			Messages: []openai.ChatCompletionMessage{
				{
					Role:    "user",
					Content: "Hello!",
				},
			},
			MaxTokens: 5,
		},
	)

	if err != nil {
		log.Printf("ChatCompletion error: %v", err)
		errorResponse := map[string]any{
			"status":     "failed",
			"error_type": "API Error",
			"message":    "Unknown error occurred",
			"timestamp":  time.Now().Format(time.RFC3339),
			"suggestions": []string{
				"Check your OPENAI_API_KEY is valid",
				"Verify your API key has necessary permissions",
				"Check if you have reached rate limits",
			},
		}
		c.JSON(http.StatusServiceUnavailable, errorResponse)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status":      "success",
		"message":     "Successfully connected to OpenAI API",
		"model_used":  config.SmallModel,
		"timestamp":   time.Now().Format(time.RFC3339),
		"response_id": resp.ID,
	})
}

// Placeholder for Root endpoint
func RootEndpoint(c *gin.Context) {
	config := core.GetConfig()
	c.JSON(http.StatusOK, gin.H{
		"message": "Claude-to-OpenAI API Proxy v1.0.0",
		"status":  "running",
		"config": gin.H{
			"openai_base_url":           config.OpenAIBaseURL,
			"max_tokens_limit":          config.MaxTokensLimit,
			"api_key_configured":        config.ValidateAPIKey(),
			"client_api_key_validation": config.AnthropicAPIKey != "",
			"big_model":                 config.BigModel,
			"small_model":               config.SmallModel,
		},
		"endpoints": gin.H{
			"messages":        "/v1/messages",
			"count_tokens":    "/v1/messages/count_tokens",
			"health":          "/health",
			"test_connection": "/test-connection",
		},
	})
}
