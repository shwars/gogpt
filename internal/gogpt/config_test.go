package gogpt

import (
	"os"
	"testing"
)

func TestParseConfigAndFindDefaults(t *testing.T) {
	cfg, err := ParseConfig(`
# comments are ignored
model "fast" {
  model_name = "gpt-fast"
  base_url = "https://example.com/v1"
  api_key = "%TEST_KEY%"
  headers = { "X-Test": "%TEST_HEADER%" }
  params = { "reasoning": { "effort": "none" }, "tools": [{ "type": "x" }] }
}

model "default" {
  model_name = "gpt-default"
  api_key = "key"
  default = true
}

template "wrap" = """
prefix {param_1}: {}
"""

system "expert" = "Be exact."
`)
	if err != nil {
		t.Fatal(err)
	}
	if len(cfg.Models) != 2 || len(cfg.Templates) != 1 || len(cfg.SystemMessages) != 1 {
		t.Fatalf("unexpected config counts: %+v", cfg)
	}
	if got := FindModel(cfg.Models, "").Name; got != "default" {
		t.Fatalf("default model = %q", got)
	}
	if got := FindModel(cfg.Models, "fast").ModelName; got != "gpt-fast" {
		t.Fatalf("named model = %q", got)
	}
	if got := FindTemplate(cfg.Templates, "wrap").Template; got != "prefix {param_1}: {}" {
		t.Fatalf("template = %q", got)
	}
	if got, ok := FindSystem(cfg.SystemMessages, "expert"); !ok || got != "Be exact." {
		t.Fatalf("system = %q %v", got, ok)
	}
}

func TestExpandEnv(t *testing.T) {
	t.Setenv("TEST_KEY", "secret")
	t.Setenv("TEST_HEADER", "header-value")
	cfg := Config{Models: []ModelConfig{{
		Name:      "m",
		ModelName: "gpt://%TEST_KEY%/model",
		APIKey:    "%TEST_KEY%",
		Headers:   map[string]string{"X-Test": "%TEST_HEADER%"},
		Params:    map[string]any{"nested": []any{"%TEST_KEY%", map[string]any{"k": "%MISSING%"}}},
	}}}
	ExpandEnv(&cfg)
	model := cfg.Models[0]
	if model.APIKey != "secret" || model.ModelName != "gpt://secret/model" {
		t.Fatalf("env expansion failed: %+v", model)
	}
	if model.Headers["X-Test"] != "header-value" {
		t.Fatalf("header expansion failed: %+v", model.Headers)
	}
	nested := model.Params["nested"].([]any)
	if nested[0] != "secret" {
		t.Fatalf("nested expansion failed: %+v", nested)
	}
	if nested[1].(map[string]any)["k"] != "%MISSING%" {
		t.Fatalf("missing env should remain unresolved: %+v", nested)
	}
}

func TestLoadConfigUsesGOGPTConfig(t *testing.T) {
	dir := t.TempDir()
	path := dir + string(os.PathSeparator) + "custom.config"
	if err := os.WriteFile(path, []byte(`model "m" { model_name = "x" api_key = "k" }`), 0o666); err != nil {
		t.Fatal(err)
	}
	t.Setenv("GOGPT_CONFIG", path)
	cfg, err := LoadConfig("")
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Path != path || FindModel(cfg.Models, "").Name != "m" {
		t.Fatalf("loaded wrong config: %+v", cfg)
	}
}

func TestValidateRejectsObsoleteClassName(t *testing.T) {
	model := ModelConfig{Name: "old", ModelName: "x", APIKey: "k", ClassName: "langchain"}
	if err := model.Validate(); err == nil {
		t.Fatal("expected obsolete classname validation error")
	}
}
