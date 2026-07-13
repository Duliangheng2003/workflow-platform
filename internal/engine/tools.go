package engine

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/schema"
)

// ——————————————————————————————————————————————————————————————
// nowTool — 获取当前日期时间
// ——————————————————————————————————————————————————————————————

type nowTool struct{}

func newNowTool() *nowTool {
	return &nowTool{}
}

func (t *nowTool) Info(ctx context.Context) (*schema.ToolInfo, error) {
	return &schema.ToolInfo{
		Name: "now",
		Desc: "Get current date and time. Returns the current time in a readable format like '2026-07-13 14:30:00 Monday'. Useful when the agent needs to know the current time, date, or day of week.",
		ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{
			"format": {
				Type:     schema.String,
				Desc:     "Optional. Time format: 'datetime' (default), 'date', 'time', 'unix'",
				Required: false,
			},
		}),
	}, nil
}

func (t *nowTool) InvokableRun(ctx context.Context, input string, opts ...tool.Option) (string, error) {
	var params struct {
		Format string `json:"format"`
	}
	json.Unmarshal([]byte(input), &params)

	now := time.Now()
	switch params.Format {
	case "date":
		return now.Format("2006-01-02"), nil
	case "time":
		return now.Format("15:04:05"), nil
	case "unix":
		return fmt.Sprintf("%d", now.Unix()), nil
	default:
		return now.Format("2006-01-02 15:04:05 Monday"), nil
	}
}

// ——————————————————————————————————————————————————————————————
// webFetchTool — 读取指定 URL 的内容
// ——————————————————————————————————————————————————————————————

type webFetchTool struct {
	client *http.Client
}

func newWebFetchTool() *webFetchTool {
	return &webFetchTool{
		client: &http.Client{Timeout: 30 * time.Second},
	}
}

func (t *webFetchTool) Info(ctx context.Context) (*schema.ToolInfo, error) {
	return &schema.ToolInfo{
		Name: "web_fetch",
		Desc: "Fetch content from a URL. Returns the page content as text. Useful for reading web pages, API responses, or any online resource.",
		ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{
			"url": {
				Type:     schema.String,
				Desc:     "The URL to fetch (must start with http:// or https://)",
				Required: true,
			},
		}),
	}, nil
}

func (t *webFetchTool) InvokableRun(ctx context.Context, input string, opts ...tool.Option) (string, error) {
	var params struct {
		URL string `json:"url"`
	}
	if err := json.Unmarshal([]byte(input), &params); err != nil {
		return "", fmt.Errorf("invalid params: %w", err)
	}
	if params.URL == "" {
		return "", fmt.Errorf("url is required")
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, params.URL, nil)
	if err != nil {
		return "", fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (compatible; WorkflowAgent/1.0)")

	resp, err := t.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("fetch failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode >= 400 {
		return "", fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(body[:min(len(body), 500)]))
	}

	// Truncate very long responses
	content := string(body)
	if len(content) > 50000 {
		content = content[:50000] + "\n... (truncated, full response was " + fmt.Sprintf("%d", len(body)) + " bytes)"
	}

	return content, nil
}

// ——————————————————————————————————————————————————————————————
// writeFileTool — 写入本地文件
// ——————————————————————————————————————————————————————————————

type writeFileTool struct {
	workDir string // working directory, defaults to current dir
}

func newWriteFileTool() *writeFileTool {
	return &writeFileTool{}
}

func (t *writeFileTool) Info(ctx context.Context) (*schema.ToolInfo, error) {
	return &schema.ToolInfo{
		Name: "write_file",
		Desc: "Write content to a file on the local filesystem. Creates directories if needed. Returns the full path of the written file.",
		ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{
			"path": {
				Type:     schema.String,
				Desc:     "File path (relative to working directory, or absolute path)",
				Required: true,
			},
			"content": {
				Type:     schema.String,
				Desc:     "Content to write to the file",
				Required: true,
			},
		}),
	}, nil
}

func (t *writeFileTool) InvokableRun(ctx context.Context, input string, opts ...tool.Option) (string, error) {
	var params struct {
		Path    string `json:"path"`
		Content string `json:"content"`
	}
	if err := json.Unmarshal([]byte(input), &params); err != nil {
		return "", fmt.Errorf("invalid params: %w", err)
	}
	if params.Path == "" {
		return "", fmt.Errorf("path is required")
	}

	// Resolve path
	filePath := params.Path
	if !filepath.IsAbs(filePath) {
		wd, err := os.Getwd()
		if err != nil {
			return "", fmt.Errorf("get working directory: %w", err)
		}
		filePath = filepath.Join(wd, filePath)
	}

	// Create parent directories
	dir := filepath.Dir(filePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", fmt.Errorf("create directories: %w", err)
	}

	// Write file
	if err := os.WriteFile(filePath, []byte(params.Content), 0644); err != nil {
		return "", fmt.Errorf("write file: %w", err)
	}

	return fmt.Sprintf("File written: %s (%d bytes)", filePath, len(params.Content)), nil
}

// ——————————————————————————————————————————————————————————————
// webSearchTool — 搜索互联网
// ——————————————————————————————————————————————————————————————

type webSearchTool struct {
	client *http.Client
}

func newWebSearchTool() *webSearchTool {
	return &webSearchTool{
		client: &http.Client{Timeout: 15 * time.Second},
	}
}

func (t *webSearchTool) Info(ctx context.Context) (*schema.ToolInfo, error) {
	return &schema.ToolInfo{
		Name: "web_search",
		Desc: "Search the internet for information. Uses DuckDuckGo to find relevant results. Returns a list of result titles, snippets, and URLs. Use this when you need current or external information.",
		ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{
			"query": {
				Type:     schema.String,
				Desc:     "Search query",
				Required: true,
			},
		}),
	}, nil
}

func (t *webSearchTool) InvokableRun(ctx context.Context, input string, opts ...tool.Option) (string, error) {
	var params struct {
		Query string `json:"query"`
	}
	if err := json.Unmarshal([]byte(input), &params); err != nil {
		return "", fmt.Errorf("invalid params: %w", err)
	}
	if params.Query == "" {
		return "", fmt.Errorf("query is required")
	}

	return t.searchDuckDuckGo(ctx, params.Query)
}

// searchDuckDuckGo uses DuckDuckGo's instant answer API (no API key required).
// Returns a formatted string of search results.
func (t *webSearchTool) searchDuckDuckGo(ctx context.Context, query string) (string, error) {
	apiURL := fmt.Sprintf("https://api.duckduckgo.com/?q=%s&format=json&no_html=1&skip_disambig=1",
		url.QueryEscape(query))

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, apiURL, nil)
	if err != nil {
		return "", fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("User-Agent", "WorkflowAgent/1.0")

	resp, err := t.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("search request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("read response: %w", err)
	}

	var result struct {
		AbstractText   string `json:"AbstractText"`
		AbstractSource string `json:"AbstractSource"`
		AbstractURL    string `json:"AbstractURL"`
		Answer         string `json:"Answer"`
		AnswerType     string `json:"AnswerType"`
		Image          string `json:"Image"`
		Results        []struct {
			FirstURL string `json:"FirstURL"`
			Text     string `json:"Text"`
		} `json:"Results"`
		RelatedTopics []struct {
			Text     string `json:"Text"`
			FirstURL string `json:"FirstURL"`
			Topics   []struct {
				Text     string `json:"Text"`
				FirstURL string `json:"FirstURL"`
			} `json:"Topics"`
		} `json:"RelatedTopics"`
	}

	if err := json.Unmarshal(body, &result); err != nil {
		return "", fmt.Errorf("parse response: %w", err)
	}

	// Build a readable result string
	var sb strings.Builder

	if result.Answer != "" {
		sb.WriteString(fmt.Sprintf("Answer: %s\n\n", result.Answer))
	}

	if result.AbstractText != "" {
		sb.WriteString(fmt.Sprintf("Summary: %s\nSource: %s\nURL: %s\n\n",
			result.AbstractText, result.AbstractSource, result.AbstractURL))
	}

	if len(result.Results) > 0 {
		sb.WriteString("Results:\n")
		for i, r := range result.Results {
			if i >= 8 {
				break
			}
			sb.WriteString(fmt.Sprintf("  %d. %s\n     %s\n", i+1, r.Text, r.FirstURL))
		}
		sb.WriteString("\n")
	}

	if len(result.RelatedTopics) > 0 {
		sb.WriteString("Related:\n")
		count := 0
		for _, rt := range result.RelatedTopics {
			if count >= 8 {
				break
			}
			if rt.Text != "" {
				sb.WriteString(fmt.Sprintf("  - %s\n", rt.Text))
				count++
			}
			for _, t := range rt.Topics {
				if count >= 8 {
					break
				}
				sb.WriteString(fmt.Sprintf("  - %s\n", t.Text))
				count++
			}
		}
	}

	if sb.Len() == 0 {
		sb.WriteString("No results found.")
	}

	return sb.String(), nil
}