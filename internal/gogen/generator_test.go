package gogen

import (
	"os"
	"path/filepath"
	"testing"
)

func testModel(baseURL string) ModelConfig {
	return ModelConfig{
		Name:      "test",
		ModelName: "test-model",
		BaseURL:   baseURL,
		APIKey:    "test-key",
		Project:   "project-1",
		Headers:   map[string]string{"X-Test": "yes"},
		Params:    map[string]any{"quality": "low", "output_format": "webp"},
	}
}

func TestBuildRequestAppliesCLIOverParams(t *testing.T) {
	generator, err := NewGenerator(testModel("https://example.com/v1"), nil)
	if err != nil {
		t.Fatal(err)
	}
	compression := 75
	request := generator.BuildRequest(GenerateOptions{
		Prompt:      "draw",
		Size:        "1536x1024",
		N:           2,
		Quality:     "high",
		Format:      "png",
		Background:  "transparent",
		Compression: &compression,
		Moderation:  "low",
		Style:       "natural",
	})
	if request["model"] != "test-model" || request["prompt"] != "draw" || request["n"] != 2 {
		t.Fatalf("request basics = %s", prettyJSON(request))
	}
	if request["quality"] != "high" || request["output_format"] != "png" || request["size"] != "1536x1024" {
		t.Fatalf("request overrides = %s", prettyJSON(request))
	}
	if request["background"] != "transparent" || request["output_compression"] != 75 || request["moderation"] != "low" || request["style"] != "natural" {
		t.Fatalf("request extras = %s", prettyJSON(request))
	}
}

func TestOutputPath(t *testing.T) {
	dir := t.TempDir()
	path, err := OutputPath("", "png", 1, 1)
	if err != nil {
		t.Fatal(err)
	}
	if path != "output.png" {
		t.Fatalf("default path = %s", path)
	}
	path, err = OutputPath(filepath.Join(dir, "image"), "webp", 1, 1)
	if err != nil {
		t.Fatal(err)
	}
	if filepath.Base(path) != "image.webp" {
		t.Fatalf("extension path = %s", path)
	}
	path, err = OutputPath(filepath.Join(dir, "out.png"), "png", 2, 2)
	if err != nil {
		t.Fatal(err)
	}
	if filepath.Base(path) != "out-2.png" {
		t.Fatalf("numbered path = %s", path)
	}
	path, err = OutputPath(dir, "jpeg", 3, 1)
	if err != nil {
		t.Fatal(err)
	}
	if filepath.Base(path) != "gogen-1.jpeg" {
		t.Fatalf("directory path = %s", path)
	}
}

func TestGenerateWritesFiles(t *testing.T) {
	server := imageTestServer(t, imageTestOptions{})
	defer server.Close()
	generator, err := NewGenerator(testModel(server.URL+"/v1"), nil)
	if err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(t.TempDir(), "image.png")
	saved, err := generator.Generate(GenerateOptions{Prompt: "draw", Out: out, N: 1, Format: "png"})
	if err != nil {
		t.Fatal(err)
	}
	if len(saved) != 1 || saved[0].Path != out || saved[0].RevisedPrompt != "revised" {
		t.Fatalf("saved = %+v", saved)
	}
	data, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "image-bytes" {
		t.Fatalf("data = %q", string(data))
	}
}

func TestGenerateWithInputImagesUsesEditEndpoint(t *testing.T) {
	server := imageTestServer(t, imageTestOptions{expectEdit: true})
	defer server.Close()
	dir := t.TempDir()
	input := filepath.Join(dir, "input.png")
	if err := os.WriteFile(input, []byte("input-image"), 0o666); err != nil {
		t.Fatal(err)
	}
	generator, err := NewGenerator(testModel(server.URL+"/v1"), nil)
	if err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(dir, "edited.png")
	saved, err := generator.Generate(GenerateOptions{
		Prompt:     "draw",
		Out:        out,
		N:          2,
		Size:       "1536x1024",
		Quality:    "high",
		ImagePaths: []string{input, input},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(saved) != 1 || saved[0].Path != out {
		t.Fatalf("saved = %+v", saved)
	}
}

func TestGenerateRejectsMissingInputImage(t *testing.T) {
	generator, err := NewGenerator(testModel("https://example.com/v1"), nil)
	if err != nil {
		t.Fatal(err)
	}
	_, err = generator.Generate(GenerateOptions{Prompt: "draw", ImagePaths: []string{filepath.Join(t.TempDir(), "missing.png")}})
	if err == nil {
		t.Fatal("expected missing image error")
	}
}

func TestGenerateRejectsUnsupportedInputImageType(t *testing.T) {
	dir := t.TempDir()
	input := filepath.Join(dir, "input.gif")
	if err := os.WriteFile(input, []byte("input-image"), 0o666); err != nil {
		t.Fatal(err)
	}
	generator, err := NewGenerator(testModel("https://example.com/v1"), nil)
	if err != nil {
		t.Fatal(err)
	}
	_, err = generator.Generate(GenerateOptions{Prompt: "draw", ImagePaths: []string{input}})
	if err == nil {
		t.Fatal("expected unsupported image type error")
	}
}
