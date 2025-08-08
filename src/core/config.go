package core

import (
	"log"
	"os"
	"strconv"
	"sync"
)

type Config struct {
	OpenAIAPIKey    string
	AnthropicAPIKey string
	OpenAIBaseURL   string
	AzureAPIVersion string
	Host            string
	Port            int
	LogLevel        string
	MaxTokensLimit  int
	MinTokensLimit  int
	RequestTimeout  int
	MaxRetries      int
	BigModel        string
	MiddleModel     string
	SmallModel      string
}

var (
	configInstance *Config
	configOnce     sync.Once
)

func GetConfig() *Config {
	configOnce.Do(func() {
		configInstance = NewConfig()
	})
	return configInstance
}

func NewConfig() *Config {
	openaiAPIKey := os.Getenv("OPENAI_API_KEY")
	if openaiAPIKey == "" {
		log.Fatalf("OPENAI_API_KEY not found in environment variables")
	}

	anthropicAPIKey := os.Getenv("ANTHROPIC_API_KEY")
	if anthropicAPIKey == "" {
		log.Println("Warning: ANTHROPIC_API_KEY not set. Client API key validation will be disabled.")
	}

	return &Config{
		OpenAIAPIKey:    openaiAPIKey,
		AnthropicAPIKey: anthropicAPIKey,
		OpenAIBaseURL:   getEnvOrDefault("OPENAI_BASE_URL", "https://api.openai.com/v1"),
		AzureAPIVersion: os.Getenv("AZURE_API_VERSION"),
		Host:            getEnvOrDefault("HOST", "0.0.0.0"),
		Port:            getEnvAsIntOrDefault("PORT", 8082),
		LogLevel:        getEnvOrDefault("LOG_LEVEL", "INFO"),
		MaxTokensLimit:  getEnvAsIntOrDefault("MAX_TOKENS_LIMIT", 4096),
		MinTokensLimit:  getEnvAsIntOrDefault("MIN_TOKENS_LIMIT", 100),
		RequestTimeout:  getEnvAsIntOrDefault("REQUEST_TIMEOUT", 90),
		MaxRetries:      getEnvAsIntOrDefault("MAX_RETRIES", 2),
		BigModel:        getEnvOrDefault("BIG_MODEL", "gpt-4o"),
		MiddleModel:     getEnvOrDefault("MIDDLE_MODEL", getEnvOrDefault("BIG_MODEL", "gpt-4o")),
		SmallModel:      getEnvOrDefault("SMALL_MODEL", "gpt-4o-mini"),
	}
}

func getEnvOrDefault(envKey, defaultValue string) string {
	if value := os.Getenv(envKey); value != "" {
		return value
	}
	return defaultValue
}

func getEnvAsIntOrDefault(envKey string, defaultValue int) int {
	if value := os.Getenv(envKey); value != "" {
		intVal, err := strconv.Atoi(value)
		if err == nil {
			return intVal
		}
	}
	return defaultValue

}

func (c *Config) ValidateAPIKey() bool {
	if c.OpenAIAPIKey == "" {
		return false
	}
	// Basic format check for OpenAI API keys
	if len(c.OpenAIAPIKey) < 3 || (c.OpenAIAPIKey[:3] != "sk-" && c.OpenAIAPIKey[:3] != "SK-") {
		return false
	}
	return true
}

func (c *Config) ValidateClientAPIKey(clientAPIKey string) bool {
	if c.AnthropicAPIKey == "" {
		return true
	}
	return clientAPIKey == c.AnthropicAPIKey
}

func (c *Config) Dump() {
	log.Println("Configuration:")
	log.Printf("OpenAIAPIKey: %s", c.OpenAIAPIKey)
	log.Printf("AnthropicAPIKey: %s", c.AnthropicAPIKey)
	log.Printf("OpenAIBaseURL: %s", c.OpenAIBaseURL)
	log.Printf("AzureAPIVersion: %s", c.AzureAPIVersion)
	log.Printf("Host: %s", c.Host)
	log.Printf("Port: %d", c.Port)
	log.Printf("LogLevel: %s", c.LogLevel)
	log.Printf("MaxTokensLimit: %d", c.MaxTokensLimit)
	log.Printf("MinTokensLimit: %d", c.MinTokensLimit)
	log.Printf("RequestTimeout: %d", c.RequestTimeout)
	log.Printf("MaxRetries: %d", c.MaxRetries)
	log.Printf("BigModel: %s", c.BigModel)
	log.Printf("MiddleModel: %s", c.MiddleModel)
	log.Printf("SmallModel: %s", c.SmallModel)
}
