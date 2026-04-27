# GPTerminal

AI-powered terminal assistant that integrates OpenAI GPT or other OpenAI API-compatible models (like Ollama) into your Linux terminal.

## Features

- **Command Fix** (`gpterminal fix` / `fuck`) - AI-corrects your last failed command
- **TUI Chat** (`gpterminal chat`) - Interactive chat with markdown rendering, one-shot stdin mode, and safe local tools
- **GPTRun** (`gpterminal run "<request>"`) - One-command AI execution with review, risk checks, and optional retry
- **GPTEdit** (`gpterminal edit <file> <instruction...>`) - AI file editing with diff preview and approval
- **GPTReview** (`gpterminal review`) - AI review for files, repo diffs, or staged diffs
- **GPTCommit** (`gpterminal commit`) - AI-generated commit messages from the staged diff
- **GPTExplainDiff** (`gpterminal explain-diff`) - Plain-language explanations of git diffs
- **GPTSessions** (`gpterminal sessions`) - List, inspect, rename, and delete saved chat or GPTDo sessions
- **Risk Evaluation** (`gpterminal risk <cmd>`) - Color-coded danger assessment of shell commands
- **Vibe Mode** (`gpterminal vibe "<description>"`) - Natural language to shell command translation
- **GPTDo** (`gpterminal gptdo "<request>"`) - Multi-step AI command execution with per-command approval
- **GPTS2T** (`gpterminal s2t <audio-file>`) - Speech-to-text transcription and optional translation
- **GPTT2S** (`gpterminal t2s "<text>"`) - Text-to-speech audio generation
- **GPTRead** (`gpterminal read [file] [question...]`) - AI analysis for text files, images, PDFs, and piped text
- **GPTImagine** (`gpterminal imagine "<prompt>"`) - Image generation with OpenAI image models
- **System-Aware** - Detects OS, kernel, shell, CPU, memory, GPU for context-aware responses

## Installation

### Pre-built binaries

Pre-built binaries for Linux and macOS are available on the [Releases](https://github.com/cycl0o0/GPTerminal/releases) page. Download the archive for your platform, extract it, and move the binary to a directory in your `PATH`:

```bash
# Example for Linux amd64
tar xzf gpterminal-linux-amd64.tar.gz
sudo mv gpterminal-linux-amd64 /usr/local/bin/gpterminal
```

```bash
# Example for macOS arm64 (Apple Silicon)
tar xzf gpterminal-darwin-arm64.tar.gz
sudo mv gpterminal-darwin-arm64 /usr/local/bin/gpterminal
```

Or, if you prefer a user-local install without `sudo`:

```bash
mkdir -p ~/.local/bin
mv gpterminal-linux-amd64 ~/.local/bin/gpterminal
```

Make sure `~/.local/bin` is in your `PATH`. Add this to your shell rc file if it isn't:

```bash
# Bash (~/.bashrc) or Zsh (~/.zshrc)
export PATH="$HOME/.local/bin:$PATH"

# Fish (~/.config/fish/config.fish)
fish_add_path ~/.local/bin
```

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

### Using with Ollama (local models)

GPTerminal supports any OpenAI-compatible API, including [Ollama](https://ollama.com). To use a local Ollama instance:

```bash
# Point GPTerminal to your Ollama server
gpterminal config set-base-url http://localhost:11434/v1

# Or use an environment variable
export OPENAI_API_BASE_URL=http://localhost:11434/v1

# No API key is required for Ollama
gpterminal chat
```

To switch back to OpenAI:

```bash
gpterminal config set-base-url https://api.openai.com/v1
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
$ gpterminal chat "summarize this error"
$ cat server.log | gpterminal chat "what are the main failures?"
$ gpterminal chat --session bugfix
$ gpterminal resume bugfix
```

With no arguments, `chat` opens the full-screen TUI with markdown rendering and system context awareness. With a prompt and/or piped stdin, it runs in one-shot mode so you can use it in shell pipelines. Use `--session <name>` to save and resume a named conversation later with `gpterminal resume <name>`.

The chat assistant can also use local tools during a conversation to inspect files, list directories, search text, stream responses, show thinking/tool status, run workspace commands directly from chat, and propose file writes with diff approval. Direct command execution in chat includes risk evaluation plus `yes` / `auto` / `no` approval, with auto-approve only available at or below the same `7/10` threshold used by GPTDo. File and path access is limited to the current working directory.

The chat TUI now shows the active session in the header and supports:
- `Ctrl+S` to save the current conversation into a named session
- `Ctrl+N` to start a fresh chat
- `Ctrl+X` to cancel the current AI response without closing the chat
- typing `exit` to quit

### GPTRun

```bash
$ gpterminal run "run the Go test suite"
```

`gptrun` generates one concrete command, shows you the command and its risk, lets you approve or edit it, then executes it. If the command fails, it can ask the AI for one retry command.

### GPTEdit

```bash
$ gpterminal edit ./README.md "add a short troubleshooting section for microphone mode"
```

`gptedit` edits one target text file at a time. It asks the AI for the full updated file content, shows a diff, and only writes after you approve it.

### GPTReview

```bash
$ gpterminal review ./internal/chatutil/runner.go
$ gpterminal review
$ gpterminal review --staged
```

`gptreview` can review a single file, the current working tree diff, or the staged diff. It prioritizes bugs, regressions, risks, and missing tests.

### GPTCommit

```bash
$ gpterminal commit
$ gpterminal commit --conventional
$ gpterminal commit --conventional --apply
```

`gptcommit` generates a commit message from the staged diff. Use `--apply` to run `git commit` with the generated message after approval.

### GPTExplainDiff

```bash
$ gpterminal explain-diff
$ gpterminal explain-diff --staged
```

`gptexplaindiff` explains what changed in a git diff, why it matters, and likely side effects.

### GPTDo

```bash
$ gpterminal gptdo "create a script called deploy.sh and make it executable"
$ gpterminal gptdo --session deploy-plan "create a script called deploy.sh and make it executable"
$ gpterminal resume deploy-plan
```

`gptdo` asks the AI for a short ordered list of commands, evaluates risk for each command, and lets you `accept`, `auto accept`, or `reject` each step. If you enable auto-accept, commands with a risk score above `7/10` still require manual confirmation. Command output is shown to you and sent back to the AI so it can continue the task. GPTDo now also saves named sessions with `--session` and shows rollback hints when the AI can propose a practical undo command.

### GPTSessions

```bash
$ gpterminal sessions
$ gpterminal sessions show bugfix
$ gpterminal sessions rename bugfix bugfix-v2
$ gpterminal sessions delete bugfix-v2
```

`gptsessions` lists saved chat and GPTDo sessions, shows details for one session, renames sessions, and deletes sessions you no longer need.

### GPTS2T

```bash
$ gpterminal s2t ./meeting.wav
$ gpterminal s2t ./interview.m4a --translate
$ gpterminal s2t ./lecture.mp3 --format srt --output ./lecture.srt
$ gpterminal s2t --mic
```

`gpts2t` is available as a shell alias after running `gpterminal init <shell>`. The command uses OpenAI speech-to-text models to transcribe supported audio files (`mp3`, `mp4`, `mpeg`, `mpga`, `m4a`, `wav`, `webm`) up to 25 MB. Use `--translate` to translate speech into English instead of keeping the original language.

**Disclaimer:** You are solely responsible for the use of this transcription feature. Ensure you have proper authorization and consent before recording or transcribing any audio. Do not use this tool to transcribe conversations without the knowledge and explicit consent of all parties involved.

For live microphone transcription, use `gpterminal s2t --mic`. This streams 24 kHz mono PCM audio to OpenAI's Realtime API and prints incremental transcript updates until you press `Ctrl+C`. Microphone mode currently supports Linux only. On Linux it now prefers `pw-record`, then `parec`, then `arecord` when ALSA exposes a real capture device, with `ffmpeg` as a final fallback. Use `--recorder` to force a backend and `--device` to target a specific source. Realtime mic mode currently supports text output only.

### GPTT2S

```bash
$ gpterminal t2s "Deploy completed successfully."
$ gpterminal t2s "Welcome to GPTerminal" --voice marin --format wav --output ./welcome.wav
$ gpterminal t2s "Read this like a calm narrator" --instructions "Speak slowly and clearly."
```

`gptt2s` is available as a shell alias after running `gpterminal init <shell>`. The command uses OpenAI text-to-speech models and saves generated audio to disk. By default it writes `speech.mp3`, and you can choose the voice, response format, instructions, and output path with flags.

### GPTRead

```bash
$ gpterminal read ./server.log "summarize the main errors"
$ gpterminal read ./diagram.png "describe this image"
$ gpterminal read ./contract.pdf "list the termination clauses"
$ gpterminal read https://example.com/docs "summarize this page"
$ cat server.log | gpterminal read "summarize the main errors"
```

`gptread` can analyze plain text files, supported images (`png`, `jpg`, `jpeg`, `gif`, `webp`), PDFs, remote URLs, and piped stdin text. For PDFs it tries local text extraction tools such as `pdftotext` or `mutool`.

### Resume And Export

```bash
$ gpterminal resume my-chat-session
$ gpterminal resume deploy-plan
$ gpterminal resume deploy-plan --export
```

`gpterminal resume <session>` resumes a named chat or GPTDo session. Use `--export` to print the saved session JSON.

### GPTImagine

```bash
$ gpterminal imagine "a retro-futuristic terminal cockpit at sunrise"
$ gpterminal imagine "minimal icon set for a CLI tool" --n 3 --size 1024x1024 --output ./artifacts
```

`gptimagine` generates images with OpenAI image models and saves them to disk. You can choose the model, image size, image count, and output directory with flags.

### Configuration

```bash
gpterminal config set-key <key>          # Save API key
gpterminal config set-base-url <url>     # Save API base URL (e.g. Ollama)
gpterminal config show                   # Show current config
```

Config is stored at `~/.config/gpterminal/config.yaml`.

## Configuration Options

| Key | Default | Env Variable | Description |
|-----|---------|-------------|-------------|
| `api_key` | - | `OPENAI_API_KEY` | OpenAI API key |
| `api_base_url` | `https://api.openai.com/v1` | `OPENAI_API_BASE_URL` | API base URL (Ollama, etc.) |
| `model` | `gpt-4o-mini` | `OPENAI_MODEL` | Model to use |
| `temperature` | `0.7` | - | Response creativity |
| `max_tokens` | `2048` | - | Max response length |

## Author

Made by [@Cycl0o0](https://github.com/cycl0o0)

## License

This project is licensed under the GNU Affero General Public License v3.0 - see the [LICENSE](LICENSE) file for details.
