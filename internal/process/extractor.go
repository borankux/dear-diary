package process

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

// Default values for the currently used OpenAI-compatible API.
const (
	defaultBaseURL = "https://api.deepseek.com"
	defaultModel   = "deepseek-chat"
)

// Extracted is the structured output from AI for one diary file.
type Extracted struct {
	Items    []CandidateExtract `json:"items"`
	Todos    []string           `json:"todos"`
	Memories []MemoryExtract    `json:"memories"`
	RawJSON  string             `json:"-"`
}

// MemoryExtract is a memory candidate before persistence.
type MemoryExtract struct {
	Topic   string `json:"topic"`
	Summary string `json:"summary"`
}

// CandidateExtract is a provider-neutral candidate item returned by the LLM.
type CandidateExtract struct {
	Type         string  `json:"type"`
	Title        string  `json:"title"`
	Content      string  `json:"content"`
	Topic        string  `json:"topic"`
	Summary      string  `json:"summary"`
	EvidenceText string  `json:"evidence_text"`
	Confidence   float64 `json:"confidence"`
}

// Extractor calls an OpenAI-compatible LLM API to extract candidates.
type Extractor struct {
	provider string
	baseURL  string
	model    string
	apiKey   string
	client   *http.Client
}

// NewExtractor creates an extractor reading configuration from environment.
// Preferred env vars: DIARY_LLM_API_KEY, DIARY_LLM_BASE_URL, DIARY_LLM_MODEL.
// DEEPSEEK_* remains supported for compatibility with existing local setup.
func NewExtractor() (*Extractor, error) {
	key := firstEnv("DIARY_LLM_API_KEY", "DEEPSEEK_API_KEY")
	if key == "" {
		return nil, errors.New("DIARY_LLM_API_KEY not set")
	}
	baseURL := firstEnv("DIARY_LLM_BASE_URL", "DEEPSEEK_BASE_URL")
	if baseURL == "" {
		baseURL = defaultBaseURL
	}
	model := firstEnv("DIARY_LLM_MODEL", "DEEPSEEK_MODEL")
	if model == "" {
		model = defaultModel
	}
	provider := os.Getenv("DIARY_LLM_PROVIDER")
	if provider == "" {
		provider = "openai-compatible"
	}
	return &Extractor{
		provider: provider,
		baseURL:  baseURL,
		model:    model,
		apiKey:   key,
		client:   &http.Client{Timeout: 120 * time.Second},
	}, nil
}

func firstEnv(keys ...string) string {
	for _, key := range keys {
		if v := os.Getenv(key); v != "" {
			return v
		}
	}
	return ""
}

// ProviderSummary returns a non-secret description of where diary content will be sent.
func (e *Extractor) ProviderSummary() string {
	return fmt.Sprintf("%s %s via %s", e.provider, e.model, e.baseURL)
}

// Extract sends one diary file's content to the configured LLM and parses the response.
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
		return nil, fmt.Errorf("llm API error %d: %s", resp.StatusCode, string(respBody))
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
		return nil, errors.New("empty choices from llm provider")
	}

	var extracted Extracted
	if err := json.Unmarshal([]byte(apiResp.Choices[0].Message.Content), &extracted); err != nil {
		return nil, fmt.Errorf("parse extracted json: %w\nraw: %s", err, apiResp.Choices[0].Message.Content)
	}
	extracted.RawJSON = apiResp.Choices[0].Message.Content
	extracted.Normalize()
	return &extracted, nil
}

// ExtractWithSystemPrompt 使用自定义 system prompt 发送内容到 LLM 并解析响应。
func (e *Extractor) ExtractWithSystemPrompt(systemPrompt, userContent string) (*Extracted, error) {
	payload := map[string]any{
		"model":       e.model,
		"temperature": 0.2,
		"messages": []map[string]string{
			{"role": "system", "content": systemPrompt},
			{"role": "user", "content": userContent},
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
		return nil, fmt.Errorf("llm API error %d: %s", resp.StatusCode, string(respBody))
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
		return nil, errors.New("empty choices from llm provider")
	}

	var extracted Extracted
	if err := json.Unmarshal([]byte(apiResp.Choices[0].Message.Content), &extracted); err != nil {
		return nil, fmt.Errorf("parse extracted json: %w\nraw: %s", err, apiResp.Choices[0].Message.Content)
	}
	extracted.RawJSON = apiResp.Choices[0].Message.Content
	extracted.Normalize()
	return &extracted, nil
}

// Normalize converts both the v0.4 items format and legacy v0.3 todos/memories
// format into one candidate list.
func (e *Extracted) Normalize() {
	for _, todo := range e.Todos {
		if strings.TrimSpace(todo) == "" {
			continue
		}
		e.Items = append(e.Items, CandidateExtract{
			Type:    CandidateTypeTodo,
			Title:   todo,
			Content: todo,
		})
	}
	for _, memory := range e.Memories {
		if strings.TrimSpace(memory.Topic) == "" && strings.TrimSpace(memory.Summary) == "" {
			continue
		}
		e.Items = append(e.Items, CandidateExtract{
			Type:    CandidateTypeMemory,
			Title:   memory.Topic,
			Content: memory.Summary,
		})
	}
}

const systemPrompt = `你是一个日记提炼助手。请阅读用户的日记内容，提取结构化候选项。

只提取两类：
1. todo：用户需要行动、跟进、完成或归档的事情。
2. memory：未来值得复用的长期记忆、偏好、经验、项目上下文或关系型信息。

不要输出 question、decision、weekly review、graph 或其它类型。

只输出严格的 JSON，格式如下，不要任何解释：
{
  "items": [
    {
      "type": "todo",
      "title": "...",
      "content": "...",
      "evidence_text": "原文中支持该提取的短句",
      "confidence": 0.9
    },
    {
      "type": "memory",
      "title": "主题",
      "content": "摘要",
      "evidence_text": "原文中支持该记忆的短句",
      "confidence": 0.8
    }
  ]
}

如果没有有价值候选，返回 {"items": []}。`
