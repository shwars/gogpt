package gogen

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type HTTPClient interface {
	Do(*http.Request) (*http.Response, error)
}

type ImageClient struct {
	model ModelConfig
	http  HTTPClient
}

type ImageResult struct {
	Data          []byte
	RevisedPrompt string
}

func NewImageClient(model ModelConfig, httpClient HTTPClient) *ImageClient {
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
	return &ImageClient{model: model, http: httpClient}
}

func (c *ImageClient) GenerateJSON(request map[string]any) ([]ImageResult, error) {
	body, err := json.Marshal(request)
	if err != nil {
		return nil, err
	}
	req, err := c.newRequest(http.MethodPost, "/images/generations", bytes.NewReader(body))
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
		return nil, fmt.Errorf("images API returned %s: %s", resp.Status, strings.TrimSpace(string(data)))
	}
	return c.decodeImageResponse(resp.Body)
}

func (c *ImageClient) EditMultipart(request map[string]any, imagePaths []string) ([]ImageResult, error) {
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	for key, value := range request {
		if !isMultipartScalar(value) {
			continue
		}
		if err := writer.WriteField(key, fmt.Sprint(value)); err != nil {
			return nil, err
		}
	}
	for _, imagePath := range imagePaths {
		if err := addMultipartFile(writer, "image[]", imagePath); err != nil {
			return nil, err
		}
	}
	if err := writer.Close(); err != nil {
		return nil, err
	}
	req, err := c.newRequest(http.MethodPost, "/images/edits", &body)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		data, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("images API returned %s: %s", resp.Status, strings.TrimSpace(string(data)))
	}
	return c.decodeImageResponse(resp.Body)
}

func (c *ImageClient) newRequest(method, endpoint string, body io.Reader) (*http.Request, error) {
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

func (c *ImageClient) downloadURL(rawURL string) ([]byte, error) {
	req, err := http.NewRequest(http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, err
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("image download returned %s", resp.Status)
	}
	return io.ReadAll(resp.Body)
}

func (c *ImageClient) decodeImageResponse(body io.Reader) ([]ImageResult, error) {
	var decoded struct {
		Data []struct {
			B64JSON       string `json:"b64_json"`
			URL           string `json:"url"`
			RevisedPrompt string `json:"revised_prompt"`
		} `json:"data"`
	}
	if err := json.NewDecoder(body).Decode(&decoded); err != nil {
		return nil, err
	}
	var results []ImageResult
	for _, item := range decoded.Data {
		result := ImageResult{RevisedPrompt: item.RevisedPrompt}
		switch {
		case item.B64JSON != "":
			data, err := decodeBase64(item.B64JSON)
			if err != nil {
				return nil, err
			}
			result.Data = data
		case item.URL != "":
			data, err := c.downloadURL(item.URL)
			if err != nil {
				return nil, err
			}
			result.Data = data
		default:
			return nil, fmt.Errorf("image response item has neither b64_json nor url")
		}
		results = append(results, result)
	}
	if len(results) == 0 {
		return nil, fmt.Errorf("image response did not include any images")
	}
	return results, nil
}

func addMultipartFile(writer *multipart.Writer, fieldName, imagePath string) error {
	file, err := os.Open(imagePath)
	if err != nil {
		return err
	}
	defer file.Close()
	filename := filepath.Base(imagePath)
	partHeader := make(textproto.MIMEHeader)
	partHeader.Set("Content-Disposition", fmt.Sprintf(`form-data; name="%s"; filename="%s"`, escapeQuotes(fieldName), escapeQuotes(filename)))
	partHeader.Set("Content-Type", imageContentType(imagePath))
	part, err := writer.CreatePart(partHeader)
	if err != nil {
		return err
	}
	_, err = io.Copy(part, file)
	return err
}

func imageContentType(imagePath string) string {
	switch strings.ToLower(filepath.Ext(imagePath)) {
	case ".jpg", ".jpeg":
		return "image/jpeg"
	case ".png":
		return "image/png"
	case ".webp":
		return "image/webp"
	}
	if contentType := mime.TypeByExtension(filepath.Ext(imagePath)); contentType != "" {
		if idx := strings.Index(contentType, ";"); idx >= 0 {
			contentType = contentType[:idx]
		}
		return contentType
	}
	return "application/octet-stream"
}

func isSupportedImageContentType(contentType string) bool {
	switch contentType {
	case "image/jpeg", "image/png", "image/webp":
		return true
	default:
		return false
	}
}

func escapeQuotes(value string) string {
	value = strings.ReplaceAll(value, `\`, `\\`)
	return strings.ReplaceAll(value, `"`, `\"`)
}

func isMultipartScalar(value any) bool {
	switch value.(type) {
	case string, bool, int, int64, float32, float64, json.Number:
		return true
	default:
		return false
	}
}

func joinURL(base, endpoint string) (string, error) {
	u, err := url.Parse(base)
	if err != nil {
		return "", err
	}
	u.Path = strings.TrimRight(u.Path, "/") + "/" + strings.TrimLeft(endpoint, "/")
	return u.String(), nil
}
