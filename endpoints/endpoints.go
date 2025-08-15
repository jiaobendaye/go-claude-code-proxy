package endpoints

import (
	"context"
	"log"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jiaobendaye/go-claude-code-proxy/core"
	"github.com/jiaobendaye/go-claude-code-proxy/models"
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
