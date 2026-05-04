package gogen

import (
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
)

type CLIArgs struct {
	Model       string
	Out         string
	Size        string
	N           int
	Help        bool
	ListModels  bool
	Quality     string
	Format      string
	Background  string
	Compression *int
	Moderation  string
	Style       string
	ImagePaths  []string
	PromptParts []string
}

const helpText = `gogen: OpenAI-compatible image generation CLI

Usage:
  gogen [options] prompt text
  gogen [options] "prompt text"
  echo prompt text | gogen [options] -

Options:
  -m, --model NAME         Select model alias from gogen.config
  -o, --out PATH           Output file or existing directory
  -s, --size SIZE          Size or size alias from gogen.config
  -i, --image PATH         Input/reference image file; repeat for multiple images
  -n NUMBER               Number of images to generate
  -h, --help              Print this help
      --list-models        List configured model aliases and raw model names
      --quality VALUE      low, medium, high, auto, standard, or hd
      --format VALUE       png, jpeg, or webp
      --background VALUE   auto, opaque, or transparent
      --compression N      JPEG/WebP compression, 0..100
      --moderation VALUE   auto or low
      --style VALUE        vivid or natural

(c) 2026 SHWARSICO Software, Vibe Coding Dept.
(c) 2026 Dmitri Soshnikov, t.me/shwarsico
`

func Main(argv []string, stdin io.Reader, stdout, stderr io.Writer) int {
	args, err := ParseArgs(argv)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 2
	}
	if args.Help {
		fmt.Fprint(stdout, helpText)
		return 0
	}
	if args.ListModels {
		cfg, err := LoadRawConfig("")
		if err != nil {
			fmt.Fprintln(stderr, err)
			return 1
		}
		PrintModels(cfg, stdout)
		return 0
	}
	cfg, err := LoadConfig("")
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	model := FindModel(cfg.Models, args.Model)
	if model == nil {
		fmt.Fprintf(stderr, "Cannot find model %s in config\n", args.Model)
		return 1
	}
	prompt, err := Prompt(args, stdin)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	if strings.TrimSpace(prompt) == "" {
		fmt.Fprintln(stderr, "prompt is empty")
		return 1
	}
	size := ResolveSize(cfg.Sizes, args.Size)
	generator, err := NewGenerator(*model, nil)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	saved, err := generator.Generate(GenerateOptions{
		Prompt:      prompt,
		Size:        size,
		Out:         args.Out,
		N:           args.N,
		Quality:     args.Quality,
		Format:      args.Format,
		Background:  args.Background,
		Compression: args.Compression,
		Moderation:  args.Moderation,
		Style:       args.Style,
		ImagePaths:  args.ImagePaths,
	})
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	for _, item := range saved {
		fmt.Fprintf(stdout, "Saved file: %s\n", item.Path)
		if item.RevisedPrompt != "" {
			fmt.Fprintf(stdout, "Revised prompt: %s\n", item.RevisedPrompt)
		}
	}
	return 0
}

func ParseArgs(argv []string) (CLIArgs, error) {
	args := CLIArgs{N: 1}
	for i := 0; i < len(argv); i++ {
		arg := argv[i]
		if arg == "--" {
			args.PromptParts = append(args.PromptParts, argv[i+1:]...)
			return args, nil
		}
		if arg == "-" || !strings.HasPrefix(arg, "-") {
			args.PromptParts = append(args.PromptParts, argv[i:]...)
			return args, nil
		}
		inlineValue := ""
		if strings.HasPrefix(arg, "--") {
			if before, after, ok := strings.Cut(arg, "="); ok {
				arg = before
				inlineValue = after
			}
		}
		next := func() (string, error) {
			if inlineValue != "" {
				value := inlineValue
				inlineValue = ""
				return value, nil
			}
			i++
			if i >= len(argv) {
				return "", fmt.Errorf("%s requires a value", arg)
			}
			return argv[i], nil
		}
		switch arg {
		case "-m", "--model":
			v, err := next()
			if err != nil {
				return args, err
			}
			args.Model = v
		case "-o", "--out":
			v, err := next()
			if err != nil {
				return args, err
			}
			args.Out = v
		case "-s", "--size":
			v, err := next()
			if err != nil {
				return args, err
			}
			args.Size = v
		case "-i", "--image":
			v, err := next()
			if err != nil {
				return args, err
			}
			args.ImagePaths = append(args.ImagePaths, v)
		case "-n":
			v, err := next()
			if err != nil {
				return args, err
			}
			n, err := strconv.Atoi(v)
			if err != nil || n < 1 {
				return args, fmt.Errorf("-n must be a positive integer")
			}
			args.N = n
		case "-h", "--help":
			args.Help = true
		case "--list-models":
			args.ListModels = true
		case "--quality":
			v, err := next()
			if err != nil {
				return args, err
			}
			if !oneOf(v, "low", "medium", "high", "auto", "standard", "hd") {
				return args, fmt.Errorf("--quality must be one of: low, medium, high, auto, standard, hd")
			}
			args.Quality = v
		case "--format":
			v, err := next()
			if err != nil {
				return args, err
			}
			if !oneOf(v, "png", "jpeg", "webp") {
				return args, fmt.Errorf("--format must be one of: png, jpeg, webp")
			}
			args.Format = v
		case "--background":
			v, err := next()
			if err != nil {
				return args, err
			}
			if !oneOf(v, "auto", "opaque", "transparent") {
				return args, fmt.Errorf("--background must be one of: auto, opaque, transparent")
			}
			args.Background = v
		case "--compression":
			v, err := next()
			if err != nil {
				return args, err
			}
			n, err := strconv.Atoi(v)
			if err != nil || n < 0 || n > 100 {
				return args, fmt.Errorf("--compression must be an integer from 0 to 100")
			}
			args.Compression = &n
		case "--moderation":
			v, err := next()
			if err != nil {
				return args, err
			}
			if !oneOf(v, "auto", "low") {
				return args, fmt.Errorf("--moderation must be one of: auto, low")
			}
			args.Moderation = v
		case "--style":
			v, err := next()
			if err != nil {
				return args, err
			}
			if !oneOf(v, "vivid", "natural") {
				return args, fmt.Errorf("--style must be one of: vivid, natural")
			}
			args.Style = v
		default:
			return args, fmt.Errorf("unknown option %s", arg)
		}
	}
	return args, nil
}

func PrintModels(cfg Config, stdout io.Writer) {
	for _, model := range cfg.Models {
		alias := model.Name
		if model.Default {
			alias += " (default)"
		}
		fmt.Fprintf(stdout, "%s\t%s\n", alias, model.ModelName)
	}
}

func Prompt(args CLIArgs, stdin io.Reader) (string, error) {
	if len(args.PromptParts) == 0 || len(args.PromptParts) == 1 && args.PromptParts[0] == "-" {
		if stdin == nil {
			return "", nil
		}
		data, err := io.ReadAll(stdin)
		if err != nil {
			return "", err
		}
		return string(data), nil
	}
	return strings.Join(args.PromptParts, " "), nil
}

func oneOf(value string, values ...string) bool {
	for _, item := range values {
		if value == item {
			return true
		}
	}
	return false
}

func WriteSampleConfig(path string) error {
	return os.WriteFile(path, []byte(sampleConfig), 0o666)
}
