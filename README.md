# GPTerminal

AI-powered terminal assistant that integrates OpenAI GPT into your Linux terminal.

## Features

- **Command Fix** (`gpterminal fix` / `fuck`) - AI-corrects your last failed command
- **TUI Chat** (`gpterminal chat`) - Interactive chat with streaming responses and markdown rendering
- **Risk Evaluation** (`gpterminal risk <cmd>`) - Color-coded danger assessment of shell commands
- **Vibe Mode** (`gpterminal vibe "<description>"`) - Natural language to shell command translation
- **GPTDo** (`gpterminal gptdo "<request>"`) - Multi-step AI command execution with per-command approval
- **GPTRead** (`gpterminal read <file> <question...>`) - AI analysis for text files, images, and PDFs
- **GPTImagine** (`gpterminal imagine "<prompt>"`) - Image generation with OpenAI image models
- **System-Aware** - Detects OS, kernel, shell, CPU, memory, GPU for context-aware responses

## Installation

### Build from source

```bash
git clone https://github.com/cycl0o0/GPTerminal.git
cd GPTerminal
make build
sudo make install
```

### Setup

1. Set your OpenAI API key:

```bash
gpterminal config set-key YOUR_API_KEY
```

2. Add shell integration to your rc file:

```bash
# Bash (~/.bashrc)
eval "$(gpterminal init bash)"

# Zsh (~/.zshrc)
eval "$(gpterminal init zsh)"

# Fish (~/.config/fish/config.fish)
eval (gpterminal init fish)
```

## Usage

### Fix last command

```bash
$ apt install neovim
E: Could not open lock file - open (13: Permission denied)
$ fuck
Last command: apt install neovim
Suggested fix: sudo apt install neovim
Execute? [Y/n]
```

### Risk evaluation

```bash
$ gpterminal risk "rm -rf /"
Risk Score: 10/10 [DANGER]
Destroys the entire filesystem irreversibly
```

### Vibe mode

```bash
$ gpterminal vibe "find all files bigger than 1GB"
Command: find / -type f -size +1G 2>/dev/null
[Y]es / [n]o / [e]dit:
```

### Interactive chat

```bash
$ gpterminal chat
```

Opens a full-screen TUI with streaming AI responses, markdown rendering, and system context awareness.

### GPTDo

```bash
$ gpterminal gptdo "create a script called deploy.sh and make it executable"
```

`gptdo` asks the AI for a short ordered list of commands, evaluates risk for each command, and lets you `accept`, `auto accept`, or `reject` each step. If you enable auto-accept, commands with a risk score above `7/10` still require manual confirmation. Command output is shown to you and sent back to the AI so it can continue the task.

### GPTRead

```bash
$ gpterminal read ./server.log "summarize the main errors"
$ gpterminal read ./diagram.png "describe this image"
$ gpterminal read ./contract.pdf "list the termination clauses"
```

`gptread` can analyze plain text files, supported images (`png`, `jpg`, `jpeg`, `gif`, `webp`), and PDFs. For PDFs it tries local text extraction tools such as `pdftotext` or `mutool`.

### GPTImagine

```bash
$ gpterminal imagine "a retro-futuristic terminal cockpit at sunrise"
$ gpterminal imagine "minimal icon set for a CLI tool" --n 3 --size 1024x1024 --output ./artifacts
```

`gptimagine` generates images with OpenAI image models and saves them to disk. You can choose the model, image size, image count, and output directory with flags.

### Configuration

```bash
gpterminal config set-key <key>   # Save API key
gpterminal config show            # Show current config
```

Config is stored at `~/.config/gpterminal/config.yaml`.

## Configuration Options

| Key | Default | Env Variable | Description |
|-----|---------|-------------|-------------|
| `api_key` | - | `OPENAI_API_KEY` | OpenAI API key |
| `model` | `gpt-4o-mini` | `OPENAI_MODEL` | Model to use |
| `temperature` | `0.7` | - | Response creativity |
| `max_tokens` | `2048` | - | Max response length |

## Author

Made by [@Cycl0o0](https://github.com/cycl0o0)

## License

This project is licensed under the GNU Affero General Public License v3.0 - see the [LICENSE](LICENSE) file for details.
