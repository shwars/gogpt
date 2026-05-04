package gogpt

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
)

type CLIArgs struct {
	Model       string
	Template    string
	System      string
	Temperature *float64
	Chat        bool
	Images      []string
	Web         bool
	WebDetail   string
	Code        bool
	OutputDir   string
	Stream      bool
	Help        bool
	Param1      string
	Param2      string
	Param3      string
	Query       []string
}

const helpText = `gogpt: OpenAI-compatible Responses API CLI

Usage:
  gogpt [options] prompt text
  gogpt [options] "prompt text"
  echo prompt text | gogpt [options] -

Options:
  -m, --model NAME         Select model alias from gogpt.config
  -p, --template VALUE     Template name, literal template, or @file
  -s, --system VALUE       System message name, literal message, or @file
  -t, --temperature VALUE  Model temperature
  -c, --chat               Continue into interactive chat
      --image PATH         Attach image file; repeat for multiple images
      --web                Enable web search
      --web-detail VALUE   low, med, or hi
      --code               Enable Code Interpreter
      --output-dir PATH    Directory for Code Interpreter output files
      --stream             Stream response text
  -1 VALUE                 Template parameter 1
  -2 VALUE                 Template parameter 2
  -3 VALUE                 Template parameter 3
  -h, --help               Print this help

(c) 2026 SHWARSICO Software, Vibe Coding Dept.
(c) 2026 Dmitri Soshnikov, t.me/shwarsico
`

func Main(argv []string, stdin *os.File, stdout, stderr io.Writer) int {
	args, err := ParseArgs(argv)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 2
	}
	if args.Help {
		fmt.Fprint(stdout, helpText)
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
	bot, err := NewBot(*model, nil)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	if err := ConfigureTemplate(bot, cfg, &args); err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	if err := ConfigureSystem(bot, cfg, &args, stdout); err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	for i, item := range args.Query {
		mapped, err := MapFile(item)
		if err != nil {
			fmt.Fprintln(stderr, err)
			return 1
		}
		args.Query[i] = mapped
	}

	isPipe := false
	if stdin != nil {
		if stat, err := stdin.Stat(); err == nil {
			isPipe = stat.Mode()&os.ModeCharDevice == 0
		}
	}
	if len(args.Query) == 1 && args.Query[0] == "-" || len(args.Query) == 0 && isPipe {
		data, err := io.ReadAll(stdin)
		if err != nil {
			fmt.Fprintln(stderr, err)
			return 1
		}
		if !runQuery(bot, string(data), args, true, false, stdout, stderr) {
			return 1
		}
		if !args.Chat {
			return 0
		}
	} else if len(args.Query) > 0 {
		if !runQuery(bot, strings.Join(args.Query, " "), args, true, false, stdout, stderr) {
			return 1
		}
		if !args.Chat {
			return 0
		}
	}

	reader := bufio.NewReader(stdin)
	for {
		fmt.Fprint(stdout, " U> ")
		q, err := reader.ReadString('\n')
		if err == io.EOF && q == "" {
			return 0
		}
		if err != nil && err != io.EOF {
			fmt.Fprintln(stderr, err)
			return 1
		}
		q = strings.TrimRight(q, "\r\n")
		if !runQuery(bot, q, args, false, true, stdout, stderr) {
			return 1
		}
		if err == io.EOF {
			return 0
		}
	}
}

func runQuery(bot *Bot, text string, args CLIArgs, includeImages bool, interactive bool, stdout, stderr io.Writer) bool {
	images := []string{}
	if includeImages {
		images = args.Images
	}
	output, saved, err := bot.Ask(text, RequestOptions{
		Temperature: args.Temperature,
		ImagePaths:  images,
		UseWeb:      args.Web,
		WebDetail:   args.WebDetail,
		UseCode:     args.Code,
		Stream:      args.Stream,
		OutputDir:   args.OutputDir,
	}, stdout)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return false
	}
	if !args.Stream && output != "" {
		if interactive {
			fmt.Fprintf(stdout, "AI> %s\n", output)
		} else {
			fmt.Fprintln(stdout, output)
		}
	}
	for _, path := range saved {
		fmt.Fprintf(stdout, "Saved file: %s\n", path)
	}
	return true
}

func ParseArgs(argv []string) (CLIArgs, error) {
	args := CLIArgs{WebDetail: "med", OutputDir: "."}
	for i := 0; i < len(argv); i++ {
		arg := argv[i]
		if arg == "--" {
			args.Query = append(args.Query, argv[i+1:]...)
			return args, nil
		}
		if arg == "-" || !strings.HasPrefix(arg, "-") {
			args.Query = append(args.Query, argv[i:]...)
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
		case "-p", "--template":
			v, err := next()
			if err != nil {
				return args, err
			}
			args.Template = v
		case "-s", "--system":
			v, err := next()
			if err != nil {
				return args, err
			}
			args.System = v
		case "-t", "--temperature":
			v, err := next()
			if err != nil {
				return args, err
			}
			f, err := parseFloat(v)
			if err != nil {
				return args, fmt.Errorf("invalid temperature %q", v)
			}
			args.Temperature = &f
		case "-c", "--chat":
			args.Chat = true
		case "--image":
			v, err := next()
			if err != nil {
				return args, err
			}
			args.Images = append(args.Images, v)
		case "--web":
			args.Web = true
		case "--web-detail":
			v, err := next()
			if err != nil {
				return args, err
			}
			if v != "low" && v != "med" && v != "hi" {
				return args, fmt.Errorf("--web-detail must be one of: low, med, hi")
			}
			args.WebDetail = v
		case "--code":
			args.Code = true
		case "--output-dir":
			v, err := next()
			if err != nil {
				return args, err
			}
			args.OutputDir = v
		case "--stream":
			args.Stream = true
		case "-h", "--help":
			args.Help = true
		case "-1":
			v, err := next()
			if err != nil {
				return args, err
			}
			args.Param1 = v
		case "-2":
			v, err := next()
			if err != nil {
				return args, err
			}
			args.Param2 = v
		case "-3":
			v, err := next()
			if err != nil {
				return args, err
			}
			args.Param3 = v
		default:
			return args, fmt.Errorf("unknown option %s", arg)
		}
	}
	return args, nil
}

func ConfigureTemplate(bot *Bot, cfg Config, args *CLIArgs) error {
	if args.Template == "" {
		return nil
	}
	var tmpl string
	if strings.HasPrefix(args.Template, "@") {
		data, err := os.ReadFile(args.Template[1:])
		if err != nil {
			return err
		}
		tmpl = string(data)
	} else if found := FindTemplate(cfg.Templates, args.Template); found != nil {
		tmpl = found.Template
	} else {
		if !strings.Contains(args.Template, "{}") {
			if !strings.Contains(args.Template, " ") {
				fmt.Printf("WARNING: Using %s as verbatim template, such name is not in config\n", args.Template)
			}
			args.Template += " \n{}"
		}
		tmpl = args.Template
	}
	params := []string{args.Param1, args.Param2, args.Param3}
	for i, value := range params {
		if value != "" {
			tmpl = strings.ReplaceAll(tmpl, fmt.Sprintf("{param_%d}", i+1), value)
		}
	}
	bot.SetPromptTranslator(func(x string) string { return strings.ReplaceAll(tmpl, "{}", x) })
	return nil
}

func ConfigureSystem(bot *Bot, cfg Config, args *CLIArgs, stdout io.Writer) error {
	if args.System == "" {
		return nil
	}
	var message string
	if strings.HasPrefix(args.System, "@") {
		data, err := os.ReadFile(args.System[1:])
		if err != nil {
			return err
		}
		message = string(data)
	} else if found, ok := FindSystem(cfg.SystemMessages, args.System); ok {
		message = found
	} else {
		if !strings.Contains(args.System, " ") {
			if stdout != nil {
				fmt.Fprintf(stdout, "WARNING: Using %s as verbatim system message, since no such name is found in config\n", args.System)
			}
		}
		message = args.System
	}
	bot.AddSystemMessage(message)
	return nil
}

func MapFile(value string) (string, error) {
	if strings.HasPrefix(value, "@") {
		data, err := os.ReadFile(value[1:])
		if err != nil {
			return "", err
		}
		return string(data), nil
	}
	return value, nil
}

func parseFloat(value string) (float64, error) {
	return strconv.ParseFloat(value, 64)
}
