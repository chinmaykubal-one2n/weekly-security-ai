package llm

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"time"
)

const openRouterURL = "https://openrouter.ai/api/v1/chat/completions"

type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type ChatRequest struct {
	Model    string    `json:"model"`
	Messages []Message `json:"messages"`
}

type ChatResponse struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
}

func Summarize(trivyJSON string) (string, error) {
	apiKey := os.Getenv("OPENROUTER_API_KEY")
	model := os.Getenv("LLM_MODEL")

	if apiKey == "" || model == "" {
		return "", errors.New("missing OpenRouter config in environment")
	}

	// Add contextual prompt
	prompt := fmt.Sprintf(`
You are a security analyst. Summarize the following Trivy JSON scan result for terminal display.

Only output plain text.
Avoid any Markdown formatting like **, backticks, or bullet symbols like '*'.
Use simple dashes (-), colons (:), and line breaks for clarity.

Include these sections:
1. Overall Risk Level
2. Summary of Detected Vulnerabilities
3. Recommendations
4. Action Items (Critical and Best Practice)

Scan Output:
%s
`, trivyJSON)

	reqBody := ChatRequest{
		Model: model,
		Messages: []Message{
			{
				Role:    "system",
				Content: "You are a security analyst. Output must be clean, plain text only. Absolutely no Markdown like **, backticks, or bullet symbols. Use '-' and ':' for listing.",
			},
			{
				Role:    "user",
				Content: prompt,
			},
		},
	}

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequest("POST", openRouterURL, bytes.NewBuffer(jsonData))
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("X-Title", "weekly-sec-ai")
	req.Header.Set("HTTP-Referer", "http://localhost")

	client := &http.Client{Timeout: 90 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	var response ChatResponse
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return "", fmt.Errorf("failed to decode response: %w", err)
	}

	if len(response.Choices) == 0 {
		return "", errors.New("no response choices returned from LLM")
	}

	return response.Choices[0].Message.Content, nil
}
