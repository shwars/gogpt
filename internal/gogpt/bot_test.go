package gogpt

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func testModel(baseURL string) ModelConfig {
	return ModelConfig{
		Name:      "test",
		ModelName: "test-model",
		BaseURL:   baseURL,
		APIKey:    "test-key",
		Params:    map[string]any{"reasoning": map[string]any{"effort": "none"}},
	}
}

func TestBuildRequestWithTemplateSystemImageWebCode(t *testing.T) {
	image := filepath.Join(t.TempDir(), "sample.png")
	if err := os.WriteFile(image, []byte("\x89PNG\r\n\x1a\n"), 0o666); err != nil {
		t.Fatal(err)
	}
	bot, err := NewBot(testModel("https://example.com/v1"), nil)
	if err != nil {
		t.Fatal(err)
	}
	bot.AddSystemMessage("Be exact.")
	bot.SetPromptTranslator(func(x string) string { return "prefix: " + x })
	temp := 0.1
	request, err := bot.BuildRequest("hello", RequestOptions{
		Temperature: &temp,
		ImagePaths:  []string{image},
		UseWeb:      true,
		WebDetail:   "hi",
		UseCode:     true,
		Stream:      true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if request["model"] != "test-model" || request["instructions"] != "Be exact." || request["temperature"] != 0.1 {
		t.Fatalf("unexpected request basics: %s", PrettyJSON(request))
	}
	input := request["input"].([]map[string]any)
	content := input[0]["content"].([]map[string]any)
	if content[0]["text"] != "prefix: hello" || content[1]["type"] != "input_image" {
		t.Fatalf("unexpected image input: %#v", content)
	}
	tools := request["tools"].([]any)
	if tools[0].(map[string]any)["search_context_size"] != "high" || tools[1].(map[string]any)["type"] != "code_interpreter" {
		t.Fatalf("unexpected tools: %#v", tools)
	}
}

func TestExtractOutputText(t *testing.T) {
	response := map[string]any{
		"output": []any{
			map[string]any{"content": []any{
				map[string]any{"type": "output_text", "text": "hello"},
				map[string]any{"type": "other", "text": "skip"},
			}},
			map[string]any{"content": []any{
				map[string]any{"type": "output_text", "text": " world"},
			}},
		},
	}
	if got := ExtractOutputText(response); got != "hello world" {
		t.Fatalf("output text = %q", got)
	}
	response["output_text"] = "direct"
	if got := ExtractOutputText(response); got != "direct" {
		t.Fatalf("direct output text = %q", got)
	}
}

func TestParseResponseStream(t *testing.T) {
	stream := strings.NewReader("data: {\"type\":\"response.output_text.delta\",\"delta\":\"do\"}\n\n" +
		"data: {\"type\":\"response.output_text.delta\",\"delta\":\"ne\"}\n\n" +
		"data: {\"type\":\"response.completed\",\"response\":{\"id\":\"resp_1\",\"output_text\":\"done\"}}\n\n")
	var out bytes.Buffer
	response, err := parseResponseStream(stream, &out)
	if err != nil {
		t.Fatal(err)
	}
	if out.String() != "done\n" {
		t.Fatalf("stream output = %q", out.String())
	}
	if response["id"] != "resp_1" || response["output_text"] != "done" {
		t.Fatalf("final response = %#v", response)
	}
}

func TestAskAndDownloadGeneratedContainerFile(t *testing.T) {
	var responseRequest map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer test-key" {
			t.Fatalf("missing auth header")
		}
		switch r.URL.Path {
		case "/v1/responses":
			if err := json.NewDecoder(r.Body).Decode(&responseRequest); err != nil {
				t.Fatal(err)
			}
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{
				"id": "resp_1",
				"output_text": "ok",
				"output": [
					{"type": "code_interpreter_call", "container_id": "cntr_1"},
					{"type": "message", "content": [
						{"type": "output_text", "annotations": [
							{"type": "container_file_citation", "container_id": "cntr_1", "file_id": "cfile_1", "filename": "result.csv"}
						]}
					]}
				]
			}`))
		case "/v1/containers/cntr_1/files":
			_, _ = w.Write([]byte(`{"data":[]}`))
		case "/v1/containers/cntr_1/files/cfile_1/content":
			_, _ = w.Write([]byte("from-container"))
		default:
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
	}))
	defer server.Close()

	bot, err := NewBot(testModel(server.URL+"/v1"), nil)
	if err != nil {
		t.Fatal(err)
	}
	outDir := t.TempDir()
	output, saved, err := bot.Ask("hello", RequestOptions{UseCode: true, OutputDir: outDir}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if output != "ok" || len(saved) != 1 {
		t.Fatalf("output/saved = %q %#v", output, saved)
	}
	data, err := os.ReadFile(saved[0])
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "from-container" {
		t.Fatalf("saved data = %q", string(data))
	}
	if responseRequest["input"] != "hello" {
		t.Fatalf("request = %s", PrettyJSON(responseRequest))
	}
}

func TestDownloadGeneratedFileFallsBackToFilesAPI(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v1/containers/cntr_1/files":
			_, _ = w.Write([]byte(`{"data":[]}`))
		case "/v1/containers/cntr_1/files/cfile_1/content":
			http.Error(w, "missing", http.StatusNotFound)
		case "/v1/files/cfile_1/content":
			_, _ = w.Write([]byte("from-files"))
		default:
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
	}))
	defer server.Close()
	bot, err := NewBot(testModel(server.URL+"/v1"), nil)
	if err != nil {
		t.Fatal(err)
	}
	response := map[string]any{"output": []any{map[string]any{"content": []any{map[string]any{"annotations": []any{
		map[string]any{"type": "container_file_citation", "container_id": "cntr_1", "file_id": "cfile_1", "filename": "fallback.txt"},
	}}}}}}
	saved, err := bot.DownloadGeneratedFiles(response, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(saved[0])
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "from-files" {
		t.Fatalf("fallback data = %q", string(data))
	}
}

func TestSafeOutputPathAvoidsCollisions(t *testing.T) {
	dir := t.TempDir()
	first, err := SafeOutputPath(dir, "../result.csv")
	if err != nil {
		t.Fatal(err)
	}
	if filepath.Base(first) != "result.csv" {
		t.Fatalf("first path = %s", first)
	}
	if err := os.WriteFile(first, []byte("x"), 0o666); err != nil {
		t.Fatal(err)
	}
	second, err := SafeOutputPath(dir, "result.csv")
	if err != nil {
		t.Fatal(err)
	}
	if filepath.Base(second) != "result-1.csv" {
		t.Fatalf("second path = %s", second)
	}
}
