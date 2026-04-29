# gogpt

`gogpt` is a dependency-free Go clone of YoGPT: a small command-line utility
for calling GPT-like models through an OpenAI-compatible Responses API.

## Build

```bash
go build ./cmd/gogpt
```

This produces `gogpt.exe` on Windows. Put `gogpt.config` next to the executable,
or set `GOGPT_CONFIG` to a config file path.

## Usage

Ask a question directly:

```bash
gogpt What is the 10th digit of Pi?
```

Pipe stdin:

```bash
echo Explain this text | gogpt -
```

If stdin is piped and no query is provided, `gogpt` reads stdin automatically:

```bash
cat notes.txt | gogpt -p summarize
```

Read input from a file:

```bash
gogpt @program.py
```

Start an interactive chat:

```bash
gogpt
```

Continue chatting after an initial prompt:

```bash
gogpt -s expert -c @program.py
```

Select a configured model:

```bash
gogpt -m qwen Explain why the sky is blue
```

Use templates and parameters:

```bash
cat english.txt | gogpt -p translate -1 German -
```

Attach images:

```bash
gogpt --image screenshot.png "Describe what is wrong in this UI"
```

Enable web search or Code Interpreter:

```bash
gogpt --web --web-detail low "What changed in Python recently?"
gogpt --code --output-dir outputs "Create a CSV with the numbers 1 to 10 and their squares"
```

Stream output:

```bash
gogpt --stream "Write a short story about a command-line tool"
```

## Configuration

Config lookup order:

1. `GOGPT_CONFIG`, if set.
2. `gogpt.config` next to the executable.
3. `gogpt.config` in the current working directory.

The config format is intentionally small and parsed by `gogpt` itself:

~~~text
model "gpt" {
  model_name = "gpt-5.5"
  base_url = "https://api.openai.com/v1"
  api_key = "%OPENAI_API_KEY%"
  params = { "reasoning": { "effort": "none" } }
}

template "summarize" = """
Please, summarize the text in triple backquotes below:
```{}```
"""

system "expert" = """
You are a careful technical expert. Give precise, practical answers and call out assumptions.
"""
~~~

Environment variables can be referenced with `%NAME%` anywhere in config values.
Unknown placeholders are left unchanged.

Model fields:

- `model_name`: model identifier sent as `model`.
- `base_url`: API root, defaulting to `https://api.openai.com/v1`.
- `api_key`: bearer token.
- `project`: optional value sent as `OpenAI-Project`.
- `default`: marks the default model alias.
- `params`: raw JSON object merged into the Responses request.
- `headers`: raw JSON object of extra HTTP headers.

The checked-in `gogpt.config` carries over the current YoGPT model aliases,
templates, and system messages.
