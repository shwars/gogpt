package gogpt

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
)

var envPattern = regexp.MustCompile(`%([^%]+)%`)

type Config struct {
	Models         []ModelConfig
	Templates      []TemplateConfig
	SystemMessages []SystemMessageConfig
	Path           string
}

type ModelConfig struct {
	Name       string
	ModelName  string
	BaseURL    string
	APIKey     string
	Project    string
	Default    bool
	Params     map[string]any
	Headers    map[string]string
	Timeout    *float64
	MaxRetries *int
	ClassName  string
}

type TemplateConfig struct {
	Name     string
	Template string
}

type SystemMessageConfig struct {
	Name    string
	Message string
}

func LoadConfig(path string) (Config, error) {
	if path == "" {
		path = os.Getenv("GOGPT_CONFIG")
	}
	if path == "" {
		if exe, err := os.Executable(); err == nil {
			candidate := filepath.Join(filepath.Dir(exe), "gogpt.config")
			if _, err := os.Stat(candidate); err == nil {
				path = candidate
			}
		}
	}
	if path == "" {
		candidate := filepath.Join(".", "gogpt.config")
		if _, err := os.Stat(candidate); err == nil {
			path = candidate
		}
	}
	if path == "" {
		return Config{}, nil
	}

	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return Config{}, nil
	}
	if err != nil {
		return Config{}, err
	}
	cfg, err := ParseConfig(string(data))
	if err != nil {
		return Config{}, fmt.Errorf("cannot parse config file %s: %w", path, err)
	}
	cfg.Path = path
	ExpandEnv(&cfg)
	return cfg, nil
}

func ExpandEnv(cfg *Config) {
	for i := range cfg.Models {
		m := &cfg.Models[i]
		m.Name = expandEnvString(m.Name)
		m.ModelName = expandEnvString(m.ModelName)
		m.BaseURL = expandEnvString(m.BaseURL)
		m.APIKey = expandEnvString(m.APIKey)
		m.Project = expandEnvString(m.Project)
		m.Params = expandEnvMap(m.Params)
		for key, value := range m.Headers {
			m.Headers[key] = expandEnvString(value)
		}
	}
	for i := range cfg.Templates {
		cfg.Templates[i].Name = expandEnvString(cfg.Templates[i].Name)
		cfg.Templates[i].Template = expandEnvString(cfg.Templates[i].Template)
	}
	for i := range cfg.SystemMessages {
		cfg.SystemMessages[i].Name = expandEnvString(cfg.SystemMessages[i].Name)
		cfg.SystemMessages[i].Message = expandEnvString(cfg.SystemMessages[i].Message)
	}
}

func expandEnvString(value string) string {
	return envPattern.ReplaceAllStringFunc(value, func(match string) string {
		name := match[1 : len(match)-1]
		if v, ok := os.LookupEnv(name); ok {
			return v
		}
		return match
	})
}

func expandEnvMap(values map[string]any) map[string]any {
	if values == nil {
		return nil
	}
	out := make(map[string]any, len(values))
	for key, value := range values {
		out[key] = expandEnvAny(value)
	}
	return out
}

func expandEnvAny(value any) any {
	switch v := value.(type) {
	case string:
		return expandEnvString(v)
	case []any:
		out := make([]any, len(v))
		for i, item := range v {
			out[i] = expandEnvAny(item)
		}
		return out
	case map[string]any:
		return expandEnvMap(v)
	default:
		return value
	}
}

func FindModel(models []ModelConfig, name string) *ModelConfig {
	if len(models) == 0 {
		return nil
	}
	if name != "" {
		for i := range models {
			if models[i].Name == name {
				return &models[i]
			}
		}
		return nil
	}
	for i := range models {
		if models[i].Default {
			return &models[i]
		}
	}
	return &models[0]
}

func FindTemplate(templates []TemplateConfig, name string) *TemplateConfig {
	for i := range templates {
		if templates[i].Name == name {
			return &templates[i]
		}
	}
	return nil
}

func FindSystem(messages []SystemMessageConfig, name string) (string, bool) {
	for _, message := range messages {
		if message.Name == name {
			return message.Message, true
		}
	}
	return "", false
}

func (m ModelConfig) Validate() error {
	var missing []string
	if m.Name == "" {
		missing = append(missing, "name")
	}
	if m.ModelName == "" {
		missing = append(missing, "model_name")
	}
	if m.APIKey == "" {
		missing = append(missing, "api_key")
	}
	if m.ClassName != "" {
		return fmt.Errorf("model %s uses obsolete LangChain config. Use model_name, base_url and api_key instead", m.Name)
	}
	if len(missing) > 0 {
		name := m.Name
		if name == "" {
			name = "<unnamed>"
		}
		return fmt.Errorf("model %s is missing: %s", name, joinStrings(missing, ", "))
	}
	return nil
}

func cloneMap(value map[string]any) map[string]any {
	if value == nil {
		return map[string]any{}
	}
	data, _ := json.Marshal(value)
	out := map[string]any{}
	_ = json.Unmarshal(data, &out)
	return out
}

func joinStrings(values []string, sep string) string {
	if len(values) == 0 {
		return ""
	}
	out := values[0]
	for _, value := range values[1:] {
		out += sep + value
	}
	return out
}
