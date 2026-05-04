package gogen

import (
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

type imageTestOptions struct {
	useURL     bool
	fail       bool
	expectEdit bool
}

func imageTestServer(t *testing.T, options imageTestOptions) *httptest.Server {
	t.Helper()
	var server *httptest.Server
	server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v1/images/generations":
			if options.expectEdit {
				t.Fatalf("expected edit endpoint, got generations")
			}
			if r.Header.Get("Authorization") != "Bearer test-key" {
				t.Fatalf("auth header = %q", r.Header.Get("Authorization"))
			}
			if r.Header.Get("OpenAI-Project") != "project-1" {
				t.Fatalf("project header = %q", r.Header.Get("OpenAI-Project"))
			}
			if r.Header.Get("X-Test") != "yes" {
				t.Fatalf("custom header = %q", r.Header.Get("X-Test"))
			}
			if options.fail {
				http.Error(w, "bad request", http.StatusBadRequest)
				return
			}
			var request map[string]any
			if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
				t.Fatal(err)
			}
			if request["model"] != "test-model" || request["prompt"] != "draw" {
				t.Fatalf("request = %s", prettyJSON(request))
			}
			writeImageResponse(w, server.URL, options.useURL)
		case "/v1/images/edits":
			if !options.expectEdit {
				t.Fatalf("expected generations endpoint, got edit")
			}
			if r.Header.Get("Authorization") != "Bearer test-key" {
				t.Fatalf("auth header = %q", r.Header.Get("Authorization"))
			}
			if r.Header.Get("OpenAI-Project") != "project-1" {
				t.Fatalf("project header = %q", r.Header.Get("OpenAI-Project"))
			}
			if r.Header.Get("X-Test") != "yes" {
				t.Fatalf("custom header = %q", r.Header.Get("X-Test"))
			}
			if options.fail {
				http.Error(w, "bad request", http.StatusBadRequest)
				return
			}
			if err := r.ParseMultipartForm(32 << 20); err != nil {
				t.Fatal(err)
			}
			if r.FormValue("model") != "test-model" || r.FormValue("prompt") != "draw" {
				t.Fatalf("multipart fields model=%q prompt=%q", r.FormValue("model"), r.FormValue("prompt"))
			}
			if r.FormValue("n") != "2" || r.FormValue("size") != "1536x1024" || r.FormValue("quality") != "high" {
				t.Fatalf("multipart fields n=%q size=%q quality=%q", r.FormValue("n"), r.FormValue("size"), r.FormValue("quality"))
			}
			files := r.MultipartForm.File["image[]"]
			if len(files) != 2 {
				t.Fatalf("image[] file count = %d", len(files))
			}
			for _, fileHeader := range files {
				if got, want := fileHeader.Header.Get("Content-Type"), imageContentType(fileHeader.Filename); got != want {
					t.Fatalf("content type for %s = %q, want %q", fileHeader.Filename, got, want)
				}
				file, err := fileHeader.Open()
				if err != nil {
					t.Fatal(err)
				}
				data, err := io.ReadAll(file)
				_ = file.Close()
				if err != nil {
					t.Fatal(err)
				}
				if string(data) != "input-image" {
					t.Fatalf("uploaded image data = %q", string(data))
				}
			}
			writeImageResponse(w, server.URL, options.useURL)
		case "/image.png":
			_, _ = w.Write([]byte("url-image-bytes"))
		default:
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
	}))
	return server
}

func writeImageResponse(w http.ResponseWriter, baseURL string, useURL bool) {
	w.Header().Set("Content-Type", "application/json")
	if useURL {
		_, _ = w.Write([]byte(`{"data":[{"url":"` + baseURL + `/image.png","revised_prompt":"from url"}]}`))
		return
	}
	encoded := base64.StdEncoding.EncodeToString([]byte("image-bytes"))
	_, _ = w.Write([]byte(`{"data":[{"b64_json":"` + encoded + `","revised_prompt":"revised"}]}`))
}

func TestImageClientGenerateFromBase64(t *testing.T) {
	server := imageTestServer(t, imageTestOptions{})
	defer server.Close()
	client := NewImageClient(testModel(server.URL+"/v1"), nil)
	results, err := client.GenerateJSON(map[string]any{"model": "test-model", "prompt": "draw"})
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 || string(results[0].Data) != "image-bytes" || results[0].RevisedPrompt != "revised" {
		t.Fatalf("results = %+v", results)
	}
}

func TestImageClientGenerateFromURL(t *testing.T) {
	server := imageTestServer(t, imageTestOptions{useURL: true})
	defer server.Close()
	client := NewImageClient(testModel(server.URL+"/v1"), nil)
	results, err := client.GenerateJSON(map[string]any{"model": "test-model", "prompt": "draw"})
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 || string(results[0].Data) != "url-image-bytes" || results[0].RevisedPrompt != "from url" {
		t.Fatalf("results = %+v", results)
	}
}

func TestImageClientGenerateAPIError(t *testing.T) {
	server := imageTestServer(t, imageTestOptions{fail: true})
	defer server.Close()
	client := NewImageClient(testModel(server.URL+"/v1"), nil)
	if _, err := client.GenerateJSON(map[string]any{"model": "test-model", "prompt": "draw"}); err == nil {
		t.Fatal("expected API error")
	}
}

func TestImageClientEditMultipart(t *testing.T) {
	server := imageTestServer(t, imageTestOptions{expectEdit: true})
	defer server.Close()
	dir := t.TempDir()
	first := filepath.Join(dir, "first.jpg")
	second := filepath.Join(dir, "second.png")
	if err := os.WriteFile(first, []byte("input-image"), 0o666); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(second, []byte("input-image"), 0o666); err != nil {
		t.Fatal(err)
	}
	client := NewImageClient(testModel(server.URL+"/v1"), nil)
	results, err := client.EditMultipart(
		map[string]any{
			"model":         "test-model",
			"prompt":        "draw",
			"n":             2,
			"size":          "1536x1024",
			"quality":       "high",
			"nested_params": map[string]any{"ignored": true},
		},
		[]string{first, second},
	)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 || string(results[0].Data) != "image-bytes" {
		t.Fatalf("results = %+v", results)
	}
}
