package process

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"
)

// Default values for DeepSeek API.
const (
	defaultBaseURL = "https://api.deepseek.com"
	defaultModel   = "deepseek-chat"
)

// Extracted is the structured output from AI for one diary file.
type Extracted struct {
	Todos    []string `json:"todos"`
	Memories []MemoryExtract `json:"memories"`
}

// MemoryExtract is a memory candidate before persistence.
type MemoryExtract struct {
	Topic   string `json:"topic"`
	Summary string `json:"summary"`
}

// Extractor calls the DeepSeek API to extract todos and memories.
type Extractor struct {
	baseURL string
	model   string
	apiKey  string
	client  *http.Client
}

// NewExtractor creates an extractor reading configuration from environment.
// Env vars: DEEPSEEK_API_KEY (required), DEEPSEEK_BASE_URL (optional), DEEPSEEK_MODEL (optional).
func NewExtractor() (*Extractor, error) {
	key := os.Getenv("DEEPSEEK_API_KEY")
	if key == "" {
		return nil, errors.New("DEEPSEEK_API_KEY not set")
	}
	baseURL := os.Getenv("DEEPSEEK_BASE_URL")
	if baseURL == "" {
		baseURL = defaultBaseURL
	}
	model := os.Getenv("DEEPSEEK_MODEL")
	if model == "" {
		model = defaultModel
	}
	return &Extractor{
		baseURL: baseURL,
		model:   model,
		apiKey:  key,
		client:  &http.Client{Timeout: 120 * time.Second},
	}, nil
}

// Extract sends one diary file's content to DeepSeek and parses the response.
func (e *Extractor) Extract(content string) (*Extracted, error) {
	payload := map[string]any{
		"model":       e.model,
		"temperature": 0.2,
		"messages": []map[string]string{
			{"role": "system", "content": systemPrompt},
			{"role": "user", "content": content},
		},
		"response_format": map[string]string{"type": "json_object"},
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest("POST", e.baseURL+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+e.apiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := e.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("deepseek API error %d: %s", resp.StatusCode, string(respBody))
	}

	var apiResp struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
		Error *struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal(respBody, &apiResp); err != nil {
		return nil, err
	}
	if apiResp.Error != nil {
		return nil, errors.New(apiResp.Error.Message)
	}
	if len(apiResp.Choices) == 0 {
		return nil, errors.New("empty choices from deepseek")
	}

	var extracted Extracted
	if err := json.Unmarshal([]byte(apiResp.Choices[0].Message.Content), &extracted); err != nil {
		return nil, fmt.Errorf("parse extracted json: %w\nraw: %s", err, apiResp.Choices[0].Message.Content)
	}
	return &extracted, nil
}

const systemPrompt = `你是一个日记提炼助手。请阅读用户的日记内容，提取两类结构化资产：

1. Todos：用户提到要做、已经完成、或者状态发生变化的事情。每条用简洁的一句话描述。
2. Memories：重要的知识、发现、经验、关系型记忆。每条包含 topic（主题）和 summary（摘要）。

只输出严格的 JSON，格式如下，不要任何解释：
{
  "todos": ["...", "..."],
  "memories": [
    {"topic": "...", "summary": "..."},
    {"topic": "...", "summary": "..."}
  ]
}

如果某类为空，返回空数组。`
