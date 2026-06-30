package client

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"
)

// ── Types ───────────────────────────────────────────────────────────

type Config struct {
	BaseURL string `json:"base_url"`
	Token   string `json:"token"`
}

type Client struct {
	config Config
	http   *http.Client
}

type Stats struct {
	Todo       int    `json:"todo"`
	Memory     int    `json:"memory"`
	Candidate  int    `json:"candidate"`
	Diary      int    `json:"diary"`
	Processing string `json:"processing"`
}

type Todo struct {
	ID        int      `json:"id"`
	Text      string   `json:"text"`
	Status    string   `json:"status"`
	Priority  *int     `json:"priority,omitempty"`
	Tags      []string `json:"tags,omitempty"`
	RawInfo   string   `json:"raw_info,omitempty"`
	CreatedAt string   `json:"created_at"`
	UpdatedAt string   `json:"updated_at"`
}

type Candidate struct {
	ID        int      `json:"id"`
	Type      string   `json:"type"`
	Title     string   `json:"title"`
	Summary   string   `json:"summary"`
	Tags      []string `json:"tags,omitempty"`
	CreatedAt string   `json:"created_at"`
}

type Memory struct {
	ID        int    `json:"id"`
	Topic     string `json:"topic"`
	CreatedAt string `json:"created_at"`
}

type DiaryEntry struct {
	Date     string `json:"date"`
	Content  string `json:"content"`
	Sections int    `json:"sections"`
	Mtime    string `json:"mtime"`
}

type CalendarDay struct {
	Day       int    `json:"day"`
	Date      string `json:"date"`
	IsPadding bool   `json:"isPadding"`
	IsWritten bool   `json:"isWritten"`
	IsToday   bool   `json:"isToday"`
}

type CalendarMonth struct {
	Month string        `json:"month"`
	Title string        `json:"title"`
	Count int           `json:"count"`
	Days  []CalendarDay `json:"days"`
}

type SearchLine struct {
	LineNum int    `json:"lineNum"`
	Text    string `json:"text"`
}

type SearchResult struct {
	Date  string       `json:"date"`
	Title string       `json:"title"`
	Lines []SearchLine `json:"lines"`
}

// ── Construction & Config ─────────────────────────────────────────────

func NewClient() *Client {
	c := &Client{
		http: &http.Client{Timeout: 15 * time.Second},
	}
	_ = c.LoadConfig()
	return c
}

func (c *Client) IsConfigured() bool {
	return c.config.BaseURL != "" && c.config.Token != ""
}

func (c *Client) Config() Config {
	return c.config
}

func (c *Client) SetConfig(baseURL, token string) {
	c.config.BaseURL = baseURL
	c.config.Token = token
}

func configPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		home = "."
	}
	return filepath.Join(home, ".config", "dear-diary", "remote.json")
}

func (c *Client) SaveConfig() error {
	path := configPath()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create config dir: %w", err)
	}
	data, err := json.MarshalIndent(c.config, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal config: %w", err)
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		return fmt.Errorf("write config: %w", err)
	}
	return nil
}

func (c *Client) LoadConfig() error {
	path := configPath()
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("read config: %w", err)
	}
	if err := json.Unmarshal(data, &c.config); err != nil {
		return fmt.Errorf("parse config: %w", err)
	}
	return nil
}

// ── Auth ────────────────────────────────────────────────────────────

func (c *Client) Login(password string) error {
	body, err := json.Marshal(map[string]string{"password": password})
	if err != nil {
		return err
	}
	res, err := c.http.Post(c.config.BaseURL+"/auth/login", "application/json", bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("login request failed: %w", err)
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		return fmt.Errorf("login failed: %s", res.Status)
	}
	var result struct {
		Token string `json:"token"`
	}
	if err := json.NewDecoder(res.Body).Decode(&result); err != nil {
		return fmt.Errorf("decode login response: %w", err)
	}
	c.config.Token = result.Token
	return nil
}

// ── HTTP helpers ────────────────────────────────────────────────────

func (c *Client) get(path string, out any) error {
	req, err := http.NewRequest("GET", c.config.BaseURL+"/api"+path, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+c.config.Token)
	res, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("GET %s: %w", path, err)
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(res.Body)
		return fmt.Errorf("GET %s: %s %s", path, res.Status, string(body))
	}
	return json.NewDecoder(res.Body).Decode(out)
}

func (c *Client) post(path string, body any) error {
	var bodyReader io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return err
		}
		bodyReader = bytes.NewReader(data)
	}
	req, err := http.NewRequest("POST", c.config.BaseURL+"/api"+path, bodyReader)
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.config.Token)
	res, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("POST %s: %w", path, err)
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(res.Body)
		return fmt.Errorf("POST %s: %s %s", path, res.Status, string(b))
	}
	return nil
}

// ── API methods ───────────────────────────────────────────────────────

func (c *Client) GetStats() (*Stats, error) {
	var s Stats
	if err := c.get("/stats", &s); err != nil {
		return nil, err
	}
	return &s, nil
}

func (c *Client) GetTodos() ([]Todo, error) {
	var list []Todo
	if err := c.get("/todos", &list); err != nil {
		return nil, err
	}
	return list, nil
}

func (c *Client) GetCandidates() ([]Candidate, error) {
	var list []Candidate
	if err := c.get("/candidates", &list); err != nil {
		return nil, err
	}
	return list, nil
}

func (c *Client) GetMemories() ([]Memory, error) {
	var list []Memory
	if err := c.get("/memories", &list); err != nil {
		return nil, err
	}
	return list, nil
}

func (c *Client) GetDiaries() ([]DiaryEntry, error) {
	var list []DiaryEntry
	if err := c.get("/diaries", &list); err != nil {
		return nil, err
	}
	return list, nil
}

func (c *Client) GetDiary(date string) (*DiaryEntry, error) {
	var d DiaryEntry
	if err := c.get("/diaries/"+date, &d); err != nil {
		return nil, err
	}
	return &d, nil
}

func (c *Client) GetCalendar() ([]CalendarMonth, error) {
	var list []CalendarMonth
	if err := c.get("/calendar", &list); err != nil {
		return nil, err
	}
	return list, nil
}

func (c *Client) Search(q string) ([]SearchResult, error) {
	var list []SearchResult
	if err := c.get("/search?q="+q, &list); err != nil {
		return nil, err
	}
	return list, nil
}

func (c *Client) CreateDiary(date, content string) error {
	return c.post("/diaries", map[string]string{
		"date":    date,
		"content": content,
	})
}

func (c *Client) UpdateTodoStatus(id int, status string) error {
	return c.post(fmt.Sprintf("/todos/%d/status", id), map[string]string{"status": status})
}

func (c *Client) AcceptCandidate(id int) error {
	return c.post(fmt.Sprintf("/candidates/%d/accept", id), nil)
}

func (c *Client) RejectCandidate(id int) error {
	return c.post(fmt.Sprintf("/candidates/%d/reject", id), nil)
}
