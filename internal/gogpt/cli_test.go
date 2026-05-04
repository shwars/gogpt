package gogpt

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParseArgs(t *testing.T) {
	args, err := ParseArgs([]string{"-m", "qwen", "-p", "translate", "-1", "German", "--web", "--web-detail", "low", "--image", "a.png", "@input.txt"})
	if err != nil {
		t.Fatal(err)
	}
	if args.Model != "qwen" || args.Template != "translate" || args.Param1 != "German" || !args.Web || args.WebDetail != "low" {
		t.Fatalf("args = %+v", args)
	}
	if len(args.Images) != 1 || args.Query[0] != "@input.txt" {
		t.Fatalf("args = %+v", args)
	}
}

func TestConfigureTemplateAndSystem(t *testing.T) {
	bot, err := NewBot(ModelConfig{Name: "m", ModelName: "model", APIKey: "key"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	cfg := Config{
		Templates:      []TemplateConfig{{Name: "wrap", Template: "prefix {param_1}: {}"}},
		SystemMessages: []SystemMessageConfig{{Name: "expert", Message: "Be exact."}},
	}
	args := CLIArgs{Template: "wrap", System: "expert", Param1: "A"}
	if err := ConfigureTemplate(bot, cfg, &args); err != nil {
		t.Fatal(err)
	}
	if err := ConfigureSystem(bot, cfg, &args, nil); err != nil {
		t.Fatal(err)
	}
	request, err := bot.BuildRequest("hello", RequestOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if request["input"] != "prefix A: hello" || request["instructions"] != "Be exact." {
		t.Fatalf("request = %s", PrettyJSON(request))
	}
}

func TestMapFile(t *testing.T) {
	file := filepath.Join(t.TempDir(), "input.txt")
	if err := os.WriteFile(file, []byte("hello"), 0o666); err != nil {
		t.Fatal(err)
	}
	got, err := MapFile("@" + file)
	if err != nil {
		t.Fatal(err)
	}
	if got != "hello" {
		t.Fatalf("mapped = %q", got)
	}
}

func TestHelpOutputIncludesCopyright(t *testing.T) {
	var out, errOut bytes.Buffer
	code := Main([]string{"--help"}, nil, &out, &errOut)
	if code != 0 {
		t.Fatalf("code = %d stderr = %s", code, errOut.String())
	}
	help := out.String()
	if !strings.Contains(help, "gogpt: OpenAI-compatible Responses API CLI") {
		t.Fatalf("help = %q", help)
	}
	if !strings.Contains(help, "(c) 2026 SHWARSICO Software, Vibe Coding Dept.") ||
		!strings.Contains(help, "(c) 2026 Dmitri Soshnikov, t.me/shwarsico") {
		t.Fatalf("copyright missing from help = %q", help)
	}
}
