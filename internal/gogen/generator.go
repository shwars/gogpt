package gogen

import (
	"encoding/base64"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type GenerateOptions struct {
	Prompt      string
	Size        string
	Out         string
	N           int
	Quality     string
	Format      string
	Background  string
	Compression *int
	Moderation  string
	Style       string
	ImagePaths  []string
}

type Generator struct {
	model  ModelConfig
	client *ImageClient
}

type SavedImage struct {
	Path          string
	RevisedPrompt string
}

func NewGenerator(model ModelConfig, httpClient HTTPClient) (*Generator, error) {
	if err := model.Validate(); err != nil {
		return nil, err
	}
	return &Generator{model: model, client: NewImageClient(model, httpClient)}, nil
}

func (g *Generator) BuildRequest(options GenerateOptions) map[string]any {
	request := cloneMap(g.model.Params)
	request["model"] = g.model.ModelName
	request["prompt"] = options.Prompt
	if options.N > 0 {
		request["n"] = options.N
	}
	if options.Size != "" {
		request["size"] = options.Size
	}
	if options.Quality != "" {
		request["quality"] = options.Quality
	}
	if options.Format != "" {
		request["output_format"] = options.Format
	}
	if options.Background != "" {
		request["background"] = options.Background
	}
	if options.Compression != nil {
		request["output_compression"] = *options.Compression
	}
	if options.Moderation != "" {
		request["moderation"] = options.Moderation
	}
	if options.Style != "" {
		request["style"] = options.Style
	}
	return request
}

func (g *Generator) Generate(options GenerateOptions) ([]SavedImage, error) {
	if options.N <= 0 {
		options.N = 1
	}
	request := g.BuildRequest(options)
	var results []ImageResult
	var err error
	if len(options.ImagePaths) > 0 {
		if err := validateImagePaths(options.ImagePaths); err != nil {
			return nil, err
		}
		results, err = g.client.EditMultipart(request, options.ImagePaths)
	} else {
		results, err = g.client.GenerateJSON(request)
	}
	if err != nil {
		return nil, err
	}
	format := outputFormat(options, request)
	var saved []SavedImage
	for i, result := range results {
		path, err := OutputPath(options.Out, format, len(results), i+1)
		if err != nil {
			return saved, err
		}
		if err := os.WriteFile(path, result.Data, 0o666); err != nil {
			return saved, err
		}
		saved = append(saved, SavedImage{Path: path, RevisedPrompt: result.RevisedPrompt})
	}
	return saved, nil
}

func validateImagePaths(paths []string) error {
	for _, imagePath := range paths {
		stat, err := os.Stat(imagePath)
		if err != nil {
			return err
		}
		if stat.IsDir() {
			return fmt.Errorf("image path is a directory: %s", imagePath)
		}
		if !isSupportedImageContentType(imageContentType(imagePath)) {
			return fmt.Errorf("unsupported image file type for %s; supported formats are JPEG, PNG, and WebP", imagePath)
		}
	}
	return nil
}

func OutputPath(out, format string, total, index int) (string, error) {
	if format == "" {
		format = "png"
	}
	format = strings.TrimPrefix(format, ".")
	if out == "" {
		out = "output." + format
	}
	if stat, err := os.Stat(out); err == nil && stat.IsDir() {
		name := fmt.Sprintf("gogen-%d.%s", index, format)
		return filepath.Join(out, name), nil
	}
	dir := filepath.Dir(out)
	if dir != "." && dir != "" {
		if err := os.MkdirAll(dir, 0o777); err != nil {
			return "", err
		}
	}
	ext := filepath.Ext(out)
	if ext == "" {
		out += "." + format
		ext = "." + format
	}
	if total <= 1 {
		return out, nil
	}
	stem := strings.TrimSuffix(out, ext)
	return fmt.Sprintf("%s-%d%s", stem, index, ext), nil
}

func outputFormat(options GenerateOptions, request map[string]any) string {
	if options.Format != "" {
		return options.Format
	}
	if value, ok := request["output_format"].(string); ok && value != "" {
		return value
	}
	if options.Out != "" {
		ext := strings.TrimPrefix(filepath.Ext(options.Out), ".")
		switch ext {
		case "png", "jpeg", "jpg", "webp":
			if ext == "jpg" {
				return "jpeg"
			}
			return ext
		}
	}
	return "png"
}

func decodeBase64(value string) ([]byte, error) {
	data, err := base64.StdEncoding.DecodeString(value)
	if err == nil {
		return data, nil
	}
	return base64.RawStdEncoding.DecodeString(value)
}
