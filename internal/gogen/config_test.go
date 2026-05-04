package gogen

import (
	"os"
	"testing"
)

func TestParseConfigModelsAndSizes(t *testing.T) {
	cfg, err := ParseConfig(`
# sample
model "gpt" {
  model_name = "gpt-image-2"
  base_url = "https://example.com/v1"
  api_key = "%OPENAI_API_KEY%"
  default = true
  headers = { "X-Test": "%TEST_HEADER%" }
  params = { "quality": "low", "output_format": "webp" }
}

model "yart" {
  model_name = "art://%folder_id%/aliceai-image-art-3.0/latest"
  api_key = "%api_key%"
}

size "square" = "1024x1024"
size "wide-4k" = "3840x2160"
`)
	if err != nil {
		t.Fatal(err)
	}
	if len(cfg.Models) != 2 || len(cfg.Sizes) != 2 {
		t.Fatalf("unexpected config counts: %+v", cfg)
	}
	if got := FindModel(cfg.Models, "").Name; got != "gpt" {
		t.Fatalf("default model = %q", got)
	}
	if got := ResolveSize(cfg.Sizes, "wide-4k"); got != "3840x2160" {
		t.Fatalf("size alias = %q", got)
	}
	if got := ResolveSize(cfg.Sizes, "123x456"); got != "123x456" {
		t.Fatalf("custom size = %q", got)
	}
}

func TestExpandEnv(t *testing.T) {
	t.Setenv("OPENAI_API_KEY", "openai-key")
	t.Setenv("folder_id", "folder")
	cfg := Config{Models: []ModelConfig{{
		Name:      "gpt",
		ModelName: "art://%folder_id%/model",
		APIKey:    "%OPENAI_API_KEY%",
		Headers:   map[string]string{"X-Key": "%OPENAI_API_KEY%"},
		Params:    map[string]any{"nested": []any{"%folder_id%"}},
	}}}
	ExpandEnv(&cfg)
	model := cfg.Models[0]
	if model.APIKey != "openai-key" || model.ModelName != "art://folder/model" {
		t.Fatalf("env expansion failed: %+v", model)
	}
	if model.Headers["X-Key"] != "openai-key" {
		t.Fatalf("header expansion failed: %+v", model.Headers)
	}
	if got := model.Params["nested"].([]any)[0]; got != "folder" {
		t.Fatalf("nested expansion = %v", got)
	}
}

func TestLoadConfigUsesGOGENConfig(t *testing.T) {
	dir := t.TempDir()
	path := dir + string(os.PathSeparator) + "custom.config"
	if err := os.WriteFile(path, []byte(`model "m" { model_name = "x" api_key = "k" }`), 0o666); err != nil {
		t.Fatal(err)
	}
	t.Setenv("GOGEN_CONFIG", path)
	cfg, err := LoadConfig("")
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Path != path || FindModel(cfg.Models, "").Name != "m" {
		t.Fatalf("loaded wrong config: %+v", cfg)
	}
}

func TestSampleConfigParses(t *testing.T) {
	cfg, err := ParseConfig(sampleConfig)
	if err != nil {
		t.Fatal(err)
	}
	if got := FindModel(cfg.Models, "").ModelName; got != "gpt-image-2" {
		t.Fatalf("default sample model = %q", got)
	}
	if FindModel(cfg.Models, "yart") == nil {
		t.Fatal("sample config missing yart")
	}
	if ResolveSize(cfg.Sizes, "portrait-4k") != "2160x3840" {
		t.Fatal("sample config missing portrait-4k")
	}
}
