package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/rs/zerolog/log"
)

// AgentClient implements the LLMProvider interface for agent operations
type AgentClient struct {
	apiKey     string
	model      string
	baseURL    string
	httpClient *http.Client
}

// NewAgentClient creates a new LLM client optimized for agent operations
func NewAgentClient() (*AgentClient, error) {

	apiKey := os.Getenv("OPENROUTER_API_KEY")
	model := os.Getenv("LLM_MODEL")

	if apiKey == "" {
		return nil, fmt.Errorf("OPENROUTER_API_KEY environment variable is required")
	}
	if model == "" {
		model = "anthropic/claude-3.5-sonnet" // Default to a capable model
	}

	return &AgentClient{
		apiKey:  apiKey,
		model:   model,
		baseURL: "https://openrouter.ai/api/v1/chat/completions",
		httpClient: &http.Client{
			Timeout: 120 * time.Second, // Longer timeout for complex agent tasks
		},
	}, nil
}

// CallLLM implements the LLMProvider interface for agent operations
func (c *AgentClient) CallLLM(ctx context.Context, prompt, systemPrompt string) (string, error) {
	reqBody := ChatRequest{
		Model: c.model,
		Messages: []Message{
			{
				Role:    "system",
				Content: systemPrompt,
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

	req, err := http.NewRequestWithContext(ctx, "POST", c.baseURL, bytes.NewBuffer(jsonData))
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	req.Header.Set("X-Title", "weekly-sec-ai-agent")
	req.Header.Set("HTTP-Referer", "http://localhost")

	// Log the request for debugging (without sensitive data)
	log.Debug().
		Str("model", c.model).
		Int("prompt_length", len(prompt)).
		Msg("Making LLM request for agent operation")

	resp, err := c.httpClient.Do(req)
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
		return "", fmt.Errorf("no response choices returned from LLM")
	}

	content := response.Choices[0].Message.Content

	// Clean up the response to ensure it's valid JSON
	content = strings.TrimSpace(content)

	// Remove markdown code blocks if present
	if strings.HasPrefix(content, "```json") {
		content = strings.TrimPrefix(content, "```json")
		content = strings.TrimSuffix(content, "```")
		content = strings.TrimSpace(content)
	}
	if strings.HasPrefix(content, "```") {
		lines := strings.Split(content, "\n")
		if len(lines) > 1 {
			content = strings.Join(lines[1:len(lines)-1], "\n")
		}
	}

	log.Debug().
		Int("response_length", len(content)).
		Msg("Received LLM response for agent operation")

	return content, nil
}

// ValidateJSONResponse checks if the LLM response is valid JSON
func (c *AgentClient) ValidateJSONResponse(response string) error {
	var temp interface{}
	return json.Unmarshal([]byte(response), &temp)
}

// CallLLMWithRetry attempts the LLM call with exponential backoff retry
func (c *AgentClient) CallLLMWithRetry(ctx context.Context, prompt, systemPrompt string, maxRetries int) (string, error) {
	var lastErr error

	for attempt := 0; attempt < maxRetries; attempt++ {
		response, err := c.CallLLM(ctx, prompt, systemPrompt)
		if err == nil {
			// Validate JSON response
			if validationErr := c.ValidateJSONResponse(response); validationErr == nil {
				return response, nil
			} else {
				log.Warn().
					Err(validationErr).
					Int("attempt", attempt+1).
					Msg("Invalid JSON response from LLM, retrying")
				lastErr = fmt.Errorf("invalid JSON response: %w", validationErr)
			}
		} else {
			lastErr = err
		}

		if attempt < maxRetries-1 {
			backoff := time.Duration(attempt+1) * time.Second
			log.Warn().
				Err(lastErr).
				Int("attempt", attempt+1).
				Dur("backoff", backoff).
				Msg("LLM call failed, retrying")

			select {
			case <-ctx.Done():
				return "", ctx.Err()
			case <-time.After(backoff):
				continue
			}
		}
	}

	return "", fmt.Errorf("LLM call failed after %d attempts: %w", maxRetries, lastErr)
}
