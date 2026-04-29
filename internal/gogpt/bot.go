package gogpt

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

var webDetailMap = map[string]string{"low": "low", "med": "medium", "hi": "high"}

type Bot struct {
	model              ModelConfig
	client             *APIClient
	systemMessages     []string
	promptTranslator   func(string) string
	previousResponseID string
}

type RequestOptions struct {
	Temperature *float64
	ImagePaths  []string
	UseWeb      bool
	WebDetail   string
	UseCode     bool
	Stream      bool
	OutputDir   string
}

func NewBot(model ModelConfig, httpClient HTTPClient) (*Bot, error) {
	if err := model.Validate(); err != nil {
		return nil, err
	}
	return &Bot{
		model:            model,
		client:           NewAPIClient(model, httpClient),
		promptTranslator: func(x string) string { return x },
	}, nil
}

func (b *Bot) AddSystemMessage(message string) {
	if message != "" {
		b.systemMessages = append(b.systemMessages, message)
	}
}

func (b *Bot) SetPromptTranslator(fn func(string) string) {
	if fn == nil {
		b.promptTranslator = func(x string) string { return x }
		return
	}
	b.promptTranslator = fn
}

func (b *Bot) Instructions() string {
	var messages []string
	for _, message := range b.systemMessages {
		if message != "" {
			messages = append(messages, message)
		}
	}
	return strings.Join(messages, "\n\n")
}

func (b *Bot) BuildRequest(message string, options RequestOptions) (map[string]any, error) {
	request := cloneMap(b.model.Params)
	configuredTools, _ := request["tools"].([]any)
	delete(request, "tools")
	tools := buildTools(options.UseWeb, options.WebDetail, options.UseCode, configuredTools)
	input, err := buildInput(b.promptTranslator(message), options.ImagePaths)
	if err != nil {
		return nil, err
	}
	request["model"] = b.model.ModelName
	request["input"] = input
	request["stream"] = options.Stream
	if instructions := b.Instructions(); instructions != "" {
		request["instructions"] = instructions
	}
	if b.previousResponseID != "" {
		request["previous_response_id"] = b.previousResponseID
	}
	if len(tools) > 0 {
		request["tools"] = tools
	}
	if options.Temperature != nil {
		request["temperature"] = *options.Temperature
	}
	return request, nil
}

func (b *Bot) Ask(message string, options RequestOptions, streamOut io.Writer) (string, []string, error) {
	if options.OutputDir == "" {
		options.OutputDir = "."
	}
	request, err := b.BuildRequest(message, options)
	if err != nil {
		return "", nil, err
	}
	response, err := b.client.CreateResponse(request, streamOut)
	if err != nil {
		return "", nil, err
	}
	if id, ok := response["id"].(string); ok && id != "" {
		b.previousResponseID = id
	}
	saved, err := b.DownloadGeneratedFiles(response, options.OutputDir)
	if err != nil {
		return "", saved, err
	}
	return ExtractOutputText(response), saved, nil
}

func buildInput(message string, imagePaths []string) (any, error) {
	if len(imagePaths) == 0 {
		return message, nil
	}
	content := []map[string]any{{"type": "input_text", "text": message}}
	for _, imagePath := range imagePaths {
		item, err := encodeImage(imagePath)
		if err != nil {
			return nil, err
		}
		content = append(content, item)
	}
	return []map[string]any{{"role": "user", "content": content}}, nil
}

func encodeImage(file string) (map[string]any, error) {
	mimeType := imageMIME(file)
	if mimeType == "" || !strings.HasPrefix(mimeType, "image/") {
		return nil, fmt.Errorf("cannot determine supported image MIME type for %s", file)
	}
	data, err := os.ReadFile(file)
	if err != nil {
		return nil, err
	}
	return map[string]any{
		"type":      "input_image",
		"image_url": "data:" + mimeType + ";base64," + base64.StdEncoding.EncodeToString(data),
	}, nil
}

func buildTools(useWeb bool, webDetail string, useCode bool, configured []any) []any {
	tools := make([]any, 0, len(configured)+2)
	tools = append(tools, configured...)
	if useWeb {
		detail := webDetailMap[webDetail]
		if detail == "" {
			detail = "medium"
		}
		tools = append(tools, map[string]any{"type": "web_search", "search_context_size": detail})
	}
	if useCode {
		tools = append(tools, map[string]any{"type": "code_interpreter", "container": map[string]any{"type": "auto"}})
	}
	return tools
}

func ExtractOutputText(response any) string {
	if obj, ok := response.(map[string]any); ok {
		if text, ok := obj["output_text"].(string); ok && text != "" {
			return text
		}
		var chunks []string
		if output, ok := obj["output"].([]any); ok {
			for _, item := range output {
				itemMap, ok := item.(map[string]any)
				if !ok {
					continue
				}
				content, ok := itemMap["content"].([]any)
				if !ok {
					continue
				}
				for _, contentItem := range content {
					contentMap, ok := contentItem.(map[string]any)
					if !ok {
						continue
					}
					if contentMap["type"] == "output_text" {
						if text, ok := contentMap["text"].(string); ok {
							chunks = append(chunks, text)
						}
					}
				}
			}
		}
		return strings.Join(chunks, "")
	}
	return ""
}

type generatedFile struct {
	ContainerID string
	FileID      string
	Filename    string
}

func (b *Bot) DownloadGeneratedFiles(response map[string]any, outputDir string) ([]string, error) {
	known, containers := ExtractGeneratedFiles(response)
	files := map[string]generatedFile{}
	for _, item := range known {
		files[item.ContainerID+"\x00"+item.FileID] = item
	}
	for containerID := range containers {
		items, err := b.client.ListContainerFiles(containerID)
		if err != nil {
			continue
		}
		for _, item := range items {
			source, _ := item["source"].(string)
			if source != "" && source != "assistant" {
				continue
			}
			fileID, _ := item["id"].(string)
			if fileID == "" {
				continue
			}
			filename := fileID
			if p, ok := item["path"].(string); ok && p != "" {
				filename = filepath.Base(p)
			}
			files[containerID+"\x00"+fileID] = generatedFile{ContainerID: containerID, FileID: fileID, Filename: filename}
		}
	}
	var saved []string
	for _, item := range files {
		path, err := SafeOutputPath(outputDir, item.Filename)
		if err != nil {
			return saved, err
		}
		var data []byte
		if item.ContainerID != "" {
			data, err = b.client.DownloadContainerFile(item.ContainerID, item.FileID)
		} else {
			err = fmt.Errorf("no container id")
		}
		if err != nil {
			data, err = b.client.DownloadFile(item.FileID)
			if err != nil {
				return saved, err
			}
		}
		if err := os.WriteFile(path, data, 0o666); err != nil {
			return saved, err
		}
		saved = append(saved, path)
	}
	return saved, nil
}

func ExtractGeneratedFiles(response map[string]any) ([]generatedFile, map[string]bool) {
	files := map[string]generatedFile{}
	containers := map[string]bool{}
	WalkJSON(response, func(obj map[string]any) {
		if obj["type"] == "code_interpreter_call" {
			if containerID, ok := obj["container_id"].(string); ok && containerID != "" {
				containers[containerID] = true
			}
		}
		if obj["type"] == "container_file_citation" {
			fileID, _ := obj["file_id"].(string)
			if fileID == "" {
				return
			}
			containerID, _ := obj["container_id"].(string)
			if containerID != "" {
				containers[containerID] = true
			}
			filename, _ := obj["filename"].(string)
			if filename == "" {
				filename = fileID
			}
			files[containerID+"\x00"+fileID] = generatedFile{ContainerID: containerID, FileID: fileID, Filename: filename}
		}
	})
	out := make([]generatedFile, 0, len(files))
	for _, item := range files {
		out = append(out, item)
	}
	return out, containers
}

func WalkJSON(value any, fn func(map[string]any)) {
	switch v := value.(type) {
	case map[string]any:
		fn(v)
		for _, item := range v {
			WalkJSON(item, fn)
		}
	case []any:
		for _, item := range v {
			WalkJSON(item, fn)
		}
	}
}

func SafeOutputPath(outputDir, filename string) (string, error) {
	if err := os.MkdirAll(outputDir, 0o777); err != nil {
		return "", err
	}
	name := filepath.Base(filename)
	if name == "." || name == string(filepath.Separator) || name == "" {
		name = "output"
	}
	candidate := filepath.Join(outputDir, name)
	if _, err := os.Stat(candidate); os.IsNotExist(err) {
		return candidate, nil
	}
	ext := filepath.Ext(name)
	stem := strings.TrimSuffix(name, ext)
	for i := 1; ; i++ {
		candidate = filepath.Join(outputDir, fmt.Sprintf("%s-%d%s", stem, i, ext))
		if _, err := os.Stat(candidate); os.IsNotExist(err) {
			return candidate, nil
		}
	}
}

func PrettyJSON(value any) string {
	data, _ := json.MarshalIndent(value, "", "  ")
	return string(data)
}
