package gogen

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParseArgs(t *testing.T) {
	args, err := ParseArgs([]string{
		"-m", "gpt",
		"--out=out.webp",
		"-s", "landscape",
		"-n", "2",
		"--quality", "high",
		"--format", "webp",
		"--background", "opaque",
		"--compression", "80",
		"--moderation", "low",
		"--style", "natural",
		"-i", "a.png",
		"--image=b.png",
		"Draw", "a", "house",
	})
	if err != nil {
		t.Fatal(err)
	}
	if args.Model != "gpt" || args.Out != "out.webp" || args.Size != "landscape" || args.N != 2 {
		t.Fatalf("args = %+v", args)
	}
	if args.Quality != "high" || args.Format != "webp" || args.Background != "opaque" || *args.Compression != 80 || args.Moderation != "low" || args.Style != "natural" {
		t.Fatalf("extra args = %+v", args)
	}
	if len(args.ImagePaths) != 2 || args.ImagePaths[0] != "a.png" || args.ImagePaths[1] != "b.png" {
		t.Fatalf("image paths = %+v", args.ImagePaths)
	}
	if got := strings.Join(args.PromptParts, " "); got != "Draw a house" {
		t.Fatalf("prompt parts = %q", got)
	}
}

func TestParseArgsListModels(t *testing.T) {
	args, err := ParseArgs([]string{"--list-models"})
	if err != nil {
		t.Fatal(err)
	}
	if !args.ListModels {
		t.Fatalf("args = %+v", args)
	}
}

func TestPromptFromArgsAndStdin(t *testing.T) {
	got, err := Prompt(CLIArgs{PromptParts: []string{"hello", "world"}}, strings.NewReader("ignored"))
	if err != nil {
		t.Fatal(err)
	}
	if got != "hello world" {
		t.Fatalf("arg prompt = %q", got)
	}
	got, err = Prompt(CLIArgs{PromptParts: []string{"-"}}, strings.NewReader("from stdin"))
	if err != nil {
		t.Fatal(err)
	}
	if got != "from stdin" {
		t.Fatalf("stdin prompt = %q", got)
	}
	got, err = Prompt(CLIArgs{}, strings.NewReader("implicit stdin"))
	if err != nil {
		t.Fatal(err)
	}
	if got != "implicit stdin" {
		t.Fatalf("implicit stdin prompt = %q", got)
	}
}

func TestHelpOutput(t *testing.T) {
	var out, errOut bytes.Buffer
	code := Main([]string{"--help"}, strings.NewReader(""), &out, &errOut)
	if code != 0 {
		t.Fatalf("code = %d stderr = %s", code, errOut.String())
	}
	if !strings.Contains(out.String(), "gogen: OpenAI-compatible image generation CLI") {
		t.Fatalf("help = %q", out.String())
	}
	if !strings.Contains(out.String(), "--list-models") {
		t.Fatalf("--list-models missing from help = %q", out.String())
	}
	if !strings.Contains(out.String(), "(c) 2026 SHWARSICO Software, Vibe Coding Dept.") ||
		!strings.Contains(out.String(), "(c) 2026 Dmitri Soshnikov, t.me/shwarsico") {
		t.Fatalf("copyright missing from help = %q", out.String())
	}
}

func TestListModelsUsesRawConfigWithoutEnvExpansion(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "gogen.config")
	config := `model "gpt" {
  model_name = "gpt-image-2"
  api_key = "%OPENAI_API_KEY%"
  default = true
}

model "yart" {
  model_name = "art://%folder_id%/aliceai-image-art-3.0/latest"
  api_key = "%api_key%"
}
`
	if err := os.WriteFile(configPath, []byte(config), 0o666); err != nil {
		t.Fatal(err)
	}
	t.Setenv("GOGEN_CONFIG", configPath)
	t.Setenv("folder_id", "expanded-folder")
	var out, errOut bytes.Buffer
	code := Main([]string{"--list-models"}, strings.NewReader(""), &out, &errOut)
	if code != 0 {
		t.Fatalf("code = %d stderr = %s", code, errOut.String())
	}
	want := "gpt (default)\tgpt-image-2\nyart\tart://%folder_id%/aliceai-image-art-3.0/latest\n"
	if out.String() != want {
		t.Fatalf("models = %q, want %q", out.String(), want)
	}
}

func TestParseArgsRejectsInvalidValues(t *testing.T) {
	cases := [][]string{
		{"-n", "0"},
		{"--quality", "ultra"},
		{"--format", "gif"},
		{"--compression", "101"},
		{"--moderation", "strict"},
		{"--style", "flat"},
	}
	for _, tc := range cases {
		if _, err := ParseArgs(tc); err == nil {
			t.Fatalf("expected error for %v", tc)
		}
	}
}
