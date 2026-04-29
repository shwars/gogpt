package gogpt

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"mime"
	"net/http"
	"net/url"
	"path"
	"path/filepath"
	"strings"
	"time"
)

type HTTPClient interface {
	Do(*http.Request) (*http.Response, error)
}

type APIClient struct {
	model ModelConfig
	http  HTTPClient
}

func NewAPIClient(model ModelConfig, httpClient HTTPClient) *APIClient {
	if httpClient == nil {
		timeout := 0.0
		if model.Timeout != nil {
			timeout = *model.Timeout
		}
		client := &http.Client{}
		if timeout > 0 {
			client.Timeout = time.Duration(timeout * float64(time.Second))
		}
		httpClient = client
	}
	return &APIClient{model: model, http: httpClient}
}

func (c *APIClient) CreateResponse(request map[string]any, streamOut io.Writer) (map[string]any, error) {
	body, err := json.Marshal(request)
	if err != nil {
		return nil, err
	}
	req, err := c.newRequest(http.MethodPost, "/responses", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		data, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("responses API returned %s: %s", resp.Status, strings.TrimSpace(string(data)))
	}
	if isTrue(request["stream"]) {
		return parseResponseStream(resp.Body, streamOut)
	}
	var data map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return nil, err
	}
	return data, nil
}

func (c *APIClient) ListContainerFiles(containerID string) ([]map[string]any, error) {
	req, err := c.newRequest(http.MethodGet, "/containers/"+url.PathEscape(containerID)+"/files", nil)
	if err != nil {
		return nil, err
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("container files API returned %s", resp.Status)
	}
	var page struct {
		Data []map[string]any `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&page); err != nil {
		return nil, err
	}
	return page.Data, nil
}

func (c *APIClient) DownloadContainerFile(containerID, fileID string) ([]byte, error) {
	req, err := c.newRequest(http.MethodGet, "/containers/"+url.PathEscape(containerID)+"/files/"+url.PathEscape(fileID)+"/content", nil)
	if err != nil {
		return nil, err
	}
	return c.download(req)
}

func (c *APIClient) DownloadFile(fileID string) ([]byte, error) {
	req, err := c.newRequest(http.MethodGet, "/files/"+url.PathEscape(fileID)+"/content", nil)
	if err != nil {
		return nil, err
	}
	return c.download(req)
}

func (c *APIClient) download(req *http.Request) ([]byte, error) {
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		data, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("file API returned %s: %s", resp.Status, strings.TrimSpace(string(data)))
	}
	return io.ReadAll(resp.Body)
}

func (c *APIClient) newRequest(method, endpoint string, body io.Reader) (*http.Request, error) {
	base := c.model.BaseURL
	if base == "" {
		base = "https://api.openai.com/v1"
	}
	fullURL, err := joinURL(base, endpoint)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequest(method, fullURL, body)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+c.model.APIKey)
	req.Header.Set("Content-Type", "application/json")
	if c.model.Project != "" {
		req.Header.Set("OpenAI-Project", c.model.Project)
	}
	for key, value := range c.model.Headers {
		req.Header.Set(key, value)
	}
	return req, nil
}

func parseResponseStream(r io.Reader, out io.Writer) (map[string]any, error) {
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 0, 64*1024), 8*1024*1024)
	var dataLines []string
	var final map[string]any
	flush := func() error {
		if len(dataLines) == 0 {
			return nil
		}
		raw := strings.Join(dataLines, "\n")
		dataLines = nil
		if raw == "[DONE]" {
			return nil
		}
		var event map[string]any
		if err := json.Unmarshal([]byte(raw), &event); err != nil {
			return err
		}
		switch event["type"] {
		case "response.output_text.delta":
			if delta, ok := event["delta"].(string); ok && out != nil {
				fmt.Fprint(out, delta)
			}
		case "response.completed":
			if response, ok := event["response"].(map[string]any); ok {
				final = response
			}
		case "response.failed":
			if errValue, ok := event["error"]; ok {
				return fmt.Errorf("streaming response failed: %v", errValue)
			}
			return fmt.Errorf("streaming response failed")
		}
		return nil
	}
	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			if err := flush(); err != nil {
				return nil, err
			}
			continue
		}
		if strings.HasPrefix(line, "data:") {
			dataLines = append(dataLines, strings.TrimSpace(strings.TrimPrefix(line, "data:")))
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	if err := flush(); err != nil {
		return nil, err
	}
	if out != nil {
		fmt.Fprintln(out)
	}
	if final == nil {
		return map[string]any{}, nil
	}
	return final, nil
}

func joinURL(base, endpoint string) (string, error) {
	u, err := url.Parse(base)
	if err != nil {
		return "", err
	}
	u.Path = strings.TrimRight(u.Path, "/") + "/" + strings.TrimLeft(endpoint, "/")
	return u.String(), nil
}

func imageMIME(file string) string {
	mimeType := mime.TypeByExtension(strings.ToLower(filepath.Ext(file)))
	if idx := strings.Index(mimeType, ";"); idx >= 0 {
		mimeType = mimeType[:idx]
	}
	if mimeType == "" {
		mimeType = mime.TypeByExtension(path.Ext(file))
	}
	return mimeType
}

func isTrue(value any) bool {
	v, ok := value.(bool)
	return ok && v
}
