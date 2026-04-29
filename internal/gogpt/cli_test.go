package gogpt

import (
	"os"
	"path/filepath"
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
