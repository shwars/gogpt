---
name: gogen
description: Generate images using an installed gogen command-line utility. Use when Codex needs to create image files from text prompts, choose image generation model aliases or sizes, pass common image parameters, or save generated images through an OpenAI-compatible image generation CLI.
---

# Gogen

Use the installed `gogen` CLI to generate image files. Assume `gogen` or `gogen.exe` is available in `PATH`; do not search repository folders for the executable and do not copy a binary into the skill.

## Workflow

1. Run `gogen --list-models` first unless the user explicitly specifies a model alias.
2. Run `gogen --help` if you need to confirm the installed flags.
3. Choose a model alias with `-m` and a size or size alias with `-s` when useful.
4. Always prefer an explicit output path with `-o` so the result is easy to find.
5. After running `gogen`, verify the output file exists and report the path to the user.

## Commands

Generate one image:

```powershell
gogen -m gpt -s landscape -o output.png "A cinematic watercolor city street at night"
```

Generate multiple images:

```powershell
gogen -m gpt -s square -n 3 -o output.png "Three sticker-style robot mascots"
```

Use extra parameters:

```powershell
gogen -m gpt --quality high --format webp --background opaque --compression 90 -o product.webp "A clean product render of a glass teapot"
```

Use stdin for long prompts:

```powershell
Get-Content prompt.txt | gogen -m gpt -s portrait -o portrait.png -
```

## Options To Use

- `--list-models`: list model aliases and raw model names for the current installation.
- `-m`, `--model`: select a model alias from the active `gogen.config`.
- `-s`, `--size`: select a literal size such as `1536x1024` or an alias from config.
- `-o`, `--out`: choose the output file or existing directory.
- `-n`: choose the number of images.
- `--quality`: pass `low`, `medium`, `high`, `auto`, `standard`, or `hd`.
- `--format`: pass `png`, `jpeg`, or `webp`.
- `--background`: pass `auto`, `opaque`, or `transparent`.
- `--compression`: pass an integer from `0` to `100`.
- `--moderation`: pass `auto` or `low`.
- `--style`: pass `vivid` or `natural`.

Do not use input/reference image flags from this skill. This skill covers text-to-image generation only.
