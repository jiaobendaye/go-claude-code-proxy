package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

const base = "http://localhost:8882"
const messageURL = base + "/v1/messages"
const tokenCountURL = base + "/v1/messages/count_tokens"
const healthURL = base + "/health"
const connectionURL = base + "/test-connection"

func testBasicChat() {
	fmt.Println("Test basic chat completion.")

	payload := map[string]interface{}{
		"model":      "claude-3-5-sonnet-20241022",
		"max_tokens": 100,
		"messages": []map[string]string{
			{"role": "user", "content": "Hello, how are you?"},
		},
	}

	body, err := json.Marshal(payload)
	if err != nil {
		fmt.Println("Error marshalling JSON:", err)
		return
	}

	resp, err := http.Post(messageURL, "application/json", bytes.NewBuffer(body))
	if err != nil {
		fmt.Println("Error making POST request:", err)
		return
	}
	defer resp.Body.Close()

	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		fmt.Println("Error reading response body:", err)
		return
	}

	fmt.Println("Basic chat response:")
	fmt.Println(string(responseBody))
}

func testStreamingChat() {
	fmt.Println("Test streaming chat completion.")

	payload := map[string]interface{}{
		"model":      "claude-3-5-haiku-20241022",
		"max_tokens": 150,
		"messages": []map[string]string{
			{"role": "user", "content": "Tell me a short joke"},
		},
		"stream": true,
	}

	body, err := json.Marshal(payload)
	if err != nil {
		fmt.Println("Error marshalling JSON:", err)
		return
	}

	req, err := http.NewRequest("POST", messageURL, bytes.NewBuffer(body))
	if err != nil {
		fmt.Println("Error creating request:", err)
		return
	}
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		fmt.Println("Error making streaming POST request:", err)
		return
	}
	defer resp.Body.Close()

	fmt.Println("Streaming response:")
	content, err := io.ReadAll(resp.Body)
	if err != nil {
		fmt.Println("Error reading streaming response body:", err)
		return
	}
	fmt.Println(string(content))
}

func testFunctionCalling() {
	fmt.Println("Test function calling capability.")

	payload := map[string]interface{}{
		"model":      "claude-3-5-sonnet-20241022",
		"max_tokens": 200,
		"messages": []map[string]string{
			{"role": "user", "content": "What's the weather like in New York? Please use the weather function."},
		},
		"tools": []map[string]interface{}{
			{
				"name":        "get_weather",
				"description": "Get the current weather for a location",
				"input_schema": map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"location": map[string]string{
							"type":        "string",
							"description": "The location to get weather for",
						},
						"unit": map[string]interface{}{
							"type":        "string",
							"enum":        []string{"celsius", "fahrenheit"},
							"description": "Temperature unit",
						},
					},
					"required": []string{"location"},
				},
			},
		},
		"tool_choice": map[string]string{"type": "auto"},
	}

	body, err := json.Marshal(payload)
	if err != nil {
		fmt.Println("Error marshalling JSON:", err)
		return
	}

	resp, err := http.Post(messageURL, "application/json", bytes.NewBuffer(body))
	if err != nil {
		fmt.Println("Error making POST request:", err)
		return
	}
	defer resp.Body.Close()

	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		fmt.Println("Error reading response body:", err)
		return
	}

	fmt.Println("Function calling response:")
	fmt.Println(string(responseBody))
}

func testWithSystemMessage() {
	fmt.Println("Test with system message.")

	payload := map[string]interface{}{
		"model":      "claude-3-5-sonnet-20241022",
		"max_tokens": 100,
		"system":     "You are a helpful assistant that always responds in haiku format.",
		"messages": []map[string]string{
			{"role": "user", "content": "Explain what AI is"},
		},
	}

	body, err := json.Marshal(payload)
	if err != nil {
		fmt.Println("Error marshalling JSON:", err)
		return
	}

	resp, err := http.Post(messageURL, "application/json", bytes.NewBuffer(body))
	if err != nil {
		fmt.Println("Error making POST request:", err)
		return
	}
	defer resp.Body.Close()

	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		fmt.Println("Error reading response body:", err)
		return
	}

	fmt.Println("System message response:")
	fmt.Println(string(responseBody))
}

func testMultimodal() {
	fmt.Println("Test multimodal input (text + image).")

	sampleImage := "iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mNkYPhfDwAChAI9jU8PJAAAAASUVORK5CYII="
	payload := map[string]interface{}{
		"model":      "claude-3-5-sonnet-20241022",
		"max_tokens": 100,
		"messages": []map[string]interface{}{
			{
				"role": "user",
				"content": []map[string]interface{}{
					{"type": "text", "text": "What do you see in this image?"},
					{
						"type": "image",
						"source": map[string]string{
							"type":       "base64",
							"media_type": "image/png",
							"data":       sampleImage,
						},
					},
				},
			},
		},
	}

	body, err := json.Marshal(payload)
	if err != nil {
		fmt.Println("Error marshalling JSON:", err)
		return
	}

	resp, err := http.Post(messageURL, "application/json", bytes.NewBuffer(body))
	if err != nil {
		fmt.Println("Error making POST request:", err)
		return
	}
	defer resp.Body.Close()

	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		fmt.Println("Error reading response body:", err)
		return
	}

	fmt.Println("Multimodal response:")
	fmt.Println(string(responseBody))
}

func testConversationWithToolUse() {
	fmt.Println("Test a complete conversation with tool use and results.")

	payload := map[string]interface{}{
		"model":      "claude-3-5-sonnet-20241022",
		"max_tokens": 200,
		"messages": []map[string]string{
			{"role": "user", "content": "Calculate 25 * 4 using the calculator tool"},
		},
		"tools": []map[string]interface{}{
			{
				"name":        "calculator",
				"description": "Perform basic arithmetic calculations",
				"input_schema": map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"expression": map[string]string{
							"type":        "string",
							"description": "Mathematical expression to calculate",
						},
					},
					"required": []string{"expression"},
				},
			},
		},
	}

	body, err := json.Marshal(payload)
	if err != nil {
		fmt.Println("Error marshalling JSON:", err)
		return
	}

	resp, err := http.Post(messageURL, "application/json", bytes.NewBuffer(body))
	if err != nil {
		fmt.Println("Error making POST request:", err)
		return
	}
	defer resp.Body.Close()

	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		fmt.Println("Error reading response body:", err)
		return
	}

	fmt.Println("Tool call response:")
	fmt.Println(string(responseBody))
}

func testTokenCounting() {
	fmt.Println("Test token counting endpoint.")

	payload := map[string]interface{}{
		"model": "claude-3-5-sonnet-20241022",
		"messages": []map[string]string{
			{"role": "user", "content": "This is a test message for token counting."},
		},
	}

	body, err := json.Marshal(payload)
	if err != nil {
		fmt.Println("Error marshalling JSON:", err)
		return
	}

	resp, err := http.Post(tokenCountURL, "application/json", bytes.NewBuffer(body))
	if err != nil {
		fmt.Println("Error making POST request:", err)
		return
	}
	defer resp.Body.Close()

	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		fmt.Println("Error reading response body:", err)
		return
	}

	fmt.Println("Token count response:")
	fmt.Println(string(responseBody))
}

func testHealthAndConnection() {
	fmt.Println("Test health and connection endpoints.")

	// Health check
	healthResponse, err := http.Get(healthURL)
	if err != nil {
		fmt.Println("Error making GET request to health endpoint:", err)
		return
	}
	defer healthResponse.Body.Close()

	healthBody, err := io.ReadAll(healthResponse.Body)
	if err != nil {
		fmt.Println("Error reading health response body:", err)
		return
	}

	fmt.Println("Health check:")
	fmt.Println(string(healthBody))

	// Connection test
	connectionResponse, err := http.Get(connectionURL)
	if err != nil {
		fmt.Println("Error making GET request to connection endpoint:", err)
		return
	}
	defer connectionResponse.Body.Close()

	connectionBody, err := io.ReadAll(connectionResponse.Body)
	if err != nil {
		fmt.Println("Error reading connection response body:", err)
		return
	}

	fmt.Println("Connection test:")
	fmt.Println(string(connectionBody))
}

func main() {
	fmt.Println("🧪 Testing Claude to OpenAI Proxy")
	fmt.Println(strings.Repeat("=", 50))

	testHealthAndConnection()
	testBasicChat()
	testFunctionCalling()
	// testStreamingChat()
	testTokenCounting()
	testConversationWithToolUse()
	testWithSystemMessage()
	testMultimodal()

	fmt.Println("✅ All tests completed!")
}
