# gogpt

`gogpt` is a dependency-free Go clone of YoGPT: a small command-line utility
for calling GPT-like models through an OpenAI-compatible Responses API.

The repository also includes `gogen`, a sibling utility for image generation
through OpenAI-compatible Images API providers.

## Build

```bash
go build ./cmd/gogpt
go build ./cmd/gogen
```

This produces `gogpt.exe` and `gogen.exe` on Windows. Put each config next to
its executable, or set `GOGPT_CONFIG` / `GOGEN_CONFIG` to a config file path.

## Release binaries

GitHub Releases provide prebuilt archives for the main desktop/server
platforms:

- `gogpt-windows-amd64.zip`
- `gogpt-windows-arm64.zip`
- `gogpt-darwin-arm64.tar.gz`
- `gogpt-darwin-amd64.tar.gz`
- `gogpt-linux-amd64.tar.gz`
- `gogpt-linux-arm64.tar.gz`
- `gogen-skill.zip`

Each platform archive includes both tools, the sample configs, and this README.
The skill archive includes only the redistributable Codex skill.

To produce release assets manually without publishing a release:

1. Open the repository on GitHub.
2. Go to **Actions**.
3. Select the **Release** workflow.
4. Click **Run workflow** and choose the branch, usually `main`.
5. Download the generated `release-artifacts` artifact from the workflow run.

With GitHub CLI:

```bash
gh workflow run release.yml --ref main
gh run watch
```

To publish release assets for a major release:

1. Create and push a release tag such as `v1.0.0`.
2. Create a GitHub Release for that tag.
3. Click **Publish release**.
4. Wait for the **Release** workflow to finish.
5. The workflow attaches all platform archives and `gogen-skill.zip` to the
   release.

macOS binaries are not signed or notarized. Apple Silicon users should download
`gogpt-darwin-arm64.tar.gz`; Intel Mac users should download
`gogpt-darwin-amd64.tar.gz`. If needed, run:

```bash
chmod +x gogpt gogen
```

macOS Gatekeeper may also require allowing the binaries in System Settings.

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

## gogen

Generate an image from a prompt:

```bash
gogen Draw a quiet street at night in watercolor
```

Read the prompt from stdin:

```bash
echo Draw a quiet street at night in watercolor | gogen -
```

Select a configured model, size alias, output file, and image count:

```bash
gogen -m yart -s landscape -n 1 -o street.png "Draw a quiet street at night in watercolor"
```

Use extra image-generation parameters:

```bash
gogen --quality high --format webp --background opaque --compression 90 "A product render of a glass teapot"
```

Use input/reference images. This switches `gogen` from image generation to the
Images API edit endpoint:

```bash
gogen -i sketch.png -i palette.png -o result.png "Turn this sketch into a polished product illustration"
```

`gogen` options:

- `-m`, `--model`: model alias from `gogen.config`.
- `-o`, `--out`: output file or existing output directory.
- `-s`, `--size`: literal size such as `1536x1024`, or a size alias.
- `-i`, `--image`: input/reference image file; repeat for multiple images.
- `-n`: number of images.
- `-h`, `--help`: print help.
- `--quality`: `low`, `medium`, `high`, `auto`, `standard`, or `hd`.
- `--format`: `png`, `jpeg`, or `webp`.
- `--background`: `auto`, `opaque`, or `transparent`.
- `--compression`: integer from `0` to `100`.
- `--moderation`: `auto` or `low`.
- `--style`: `vivid` or `natural`.

`gogen` config lookup order:

1. `GOGEN_CONFIG`, if set.
2. `gogen.config` next to the executable.
3. `gogen.config` in the current working directory.

Example `gogen.config`:

~~~text
model "gpt" {
  model_name = "gpt-image-2"
  base_url = "https://api.openai.com/v1"
  api_key = "%OPENAI_API_KEY%"
  default = true
}

model "yart" {
  model_name = "art://%folder_id%/aliceai-image-art-3.0/latest"
  base_url = "https://ai.api.cloud.yandex.net/v1"
  api_key = "%api_key%"
  project = "%folder_id%"
}

size "square" = "1024x1024"
size "landscape" = "1536x1024"
size "portrait" = "1024x1536"
~~~

All providers use `Authorization: Bearer <api_key>`. If `project` is set, it is
sent as `OpenAI-Project`. The checked-in `gogen.config` includes `gpt`,
`gpt1.5`, `gpt1`, `yart`, and common size aliases.

### Codex skill

This repository includes a redistributable Codex skill for using `gogen` from
coding-agent workflows. The skill is packaged as `gogen-skill.zip` for release
uploads.

Before installing the skill, install and configure the `gogen` executable:

1. Put `gogen.exe` in a directory on `PATH`, or add the directory containing
   `gogen.exe` to `PATH`.
2. Put `gogen.config` next to `gogen.exe`, or set `GOGEN_CONFIG` to the config
   file path.
3. Verify the installation:

```powershell
gogen --list-models
```

Then install the skill by unzipping `gogen-skill.zip` into your Codex skills
directory. On Windows this is usually:

```powershell
Expand-Archive .\gogen-skill.zip -DestinationPath "$env:USERPROFILE\.codex\skills" -Force
```

After installation, the skill folder should be:

```text
%USERPROFILE%\.codex\skills\gogen
```

The skill assumes `gogen` is available on `PATH`; it does not search for
`gogen.exe` beside the skill folder.
