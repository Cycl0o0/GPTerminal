# GPTerminal

AI-powered terminal assistant that integrates OpenAI GPT, Anthropic Claude, Google Gemini, OpenClaw, or other OpenAI API-compatible models (like Ollama) into your Linux, macOS, and Windows terminal.

## Features

- **Command Fix** (`gpterminal fix` / `fuck`) - AI-corrects your last failed command
- **TUI Chat** (`gpterminal chat`) - Interactive chat with markdown rendering, one-shot stdin mode, local tools, and web search/fetch
- **GPTRun** (`gpterminal run "<request>"`) - One-command AI execution with review, risk checks, and optional retry
- **GPTEdit** (`gpterminal edit <file> <instruction...>`) - AI file editing with diff preview and approval
- **GPTReview** (`gpterminal review`) - AI review for files, repo diffs, or staged diffs
- **GPTCommit** (`gpterminal commit`) - AI-generated commit messages from the staged diff
- **GPTExplainDiff** (`gpterminal explain-diff`) - Plain-language explanations of git diffs
- **GPTSessions** (`gpterminal sessions`) - List, inspect, rename, and delete saved chat or GPTDo sessions
- **Risk Evaluation** (`gpterminal risk <cmd>`) - Color-coded danger assessment of shell commands
- **Vibe Mode** (`gpterminal vibe "<description>"`) - Natural language to shell command translation
- **GPTDo** (`gpterminal gptdo` / `do "<request>"`) - Multi-step AI command execution, gated by a local deterministic safety policy
- **Doctor** (`gpterminal doctor`) - Offline diagnostics for config and security posture, with stable `--json`
- **Eval** (`gpterminal eval`) - Deterministic regression suite for the command-safety policy (`--json`, `--fixture`)
- **Abort** (`gpterminal abort <session>`) - Remove a saved session's durable state so it can no longer be resumed
- **GPTS2T** (`gpterminal s2t <audio-file>`) - Speech-to-text transcription and optional translation
- **GPTT2S** (`gpterminal t2s "<text>"`) - Text-to-speech audio generation
- **GPTRead** (`gpterminal read [file] [question...]`) - AI analysis for text files, images, PDFs, and piped text
- **GPTImagine** (`gpterminal imagine "<prompt>"`) - Image generation with OpenAI image models
- **Inline Suggest** (`gpterminal suggest` / `Ctrl+G`) - AI-powered inline command completion and correction
- **Agent Mode** (`gpterminal agent "<objective>"`) - Autonomous AI agent that plans and executes multi-step tasks
- **GPTCode** (`gpterminal code`) - Interactive AI coding assistant with project-aware context, slash commands, and session persistence
- **Translate** (`gpterminal translate <file> <lang>`) - Translate source code between programming languages with idiomatic output
- **AI Shell** (`gpterminal shell`) - AI-enhanced interactive shell with auto-fix on errors, `?` for questions, `!` for command generation
- **Stats Dashboard** (`gpterminal stats`) - Usage statistics with per-command tracking and optional TUI dashboard
- **Auto-Update** (`gpterminal update`) - Check for and install updates from GitHub Releases
- **Custom Templates** (`gpterminal template`) - Define custom AI commands via YAML template files
- **Web Search & Fetch** - Chat and agent can search the web (DuckDuckGo) and fetch URL content as tools
- **MCP Support** - Connect Model Context Protocol servers to extend available tools in chat and agent, with automatic reconnect on server crash
- **Multi-Provider** - Native support for OpenAI, Anthropic Claude, Google Gemini, and OpenClaw with provider-specific features (extended thinking, native tool use, server-side agents)
- **OpenClaw Integration** - Connect to an OpenClaw server for autonomous AI agent execution with server-side tools, SSE streaming, and Bearer/password authentication
- **Conversation Memory** (`gpterminal memory`) - Persistent memory across chat sessions so the AI remembers your project, preferences, and context
- **Patch-Based Editing** - `edit_file` tool for targeted search-and-replace file edits with diff preview and approval (safer than full rewrites)
- **Command Hooks** - Run custom shell commands before/after command execution via config
- **Enhanced Pipe Mode** - All commands support stdin piping and disable colors when stdout is not a TTY
- **System-Aware** - Detects OS, kernel, shell, CPU, memory, GPU for context-aware responses

## Installation

### Pre-built binaries

Pre-built binaries for Linux, macOS, and Windows are available on the [Releases](https://github.com/cycl0o0/GPTerminal/releases) page. Download the archive for your platform, extract it, and move the binary to a directory in your `PATH`:

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

```powershell
# Example for Windows amd64 (PowerShell)
tar xzf gpterminal-windows-amd64.tar.gz
Move-Item gpterminal-windows-amd64.exe C:\Users\you\bin\gpterminal.exe
```

Or, if you prefer a user-local install without `sudo` (Linux/macOS):

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

1. Choose your AI provider and set an API key:

```bash
# OpenAI (default)
gpterminal config set-provider openai
gpterminal config set-key YOUR_OPENAI_API_KEY

# Anthropic Claude
gpterminal config set-provider anthropic
gpterminal config set-anthropic-key YOUR_ANTHROPIC_API_KEY
gpterminal config set-model claude-sonnet-4-20250514

# Google Gemini
gpterminal config set-provider gemini
gpterminal config set-gemini-key YOUR_GEMINI_API_KEY
gpterminal config set-model gemini-2.0-flash

# OpenClaw (server-side AI agent)
gpterminal config set-provider openclaw
gpterminal config set-openclaw-url http://your-server:18789
gpterminal config set-openclaw-token YOUR_TOKEN
# Or use shared-password auth instead of token:
gpterminal config set-openclaw-password YOUR_PASSWORD
# Optionally set an agent name:
gpterminal config set-openclaw-agent my-agent
```

Or run the interactive setup wizard:

```bash
gpterminal setup
```

2. Add shell integration to your rc file:

```bash
# Bash (~/.bashrc)
eval "$(gpterminal init bash)"

# Zsh (~/.zshrc)
eval "$(gpterminal init zsh)"

# Fish (~/.config/fish/config.fish)
eval (gpterminal init fish)

# PowerShell ($PROFILE)
gpterminal init powershell | Invoke-Expression
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

Use `--yes`/`-y` to auto-approve the generated command (useful in scripts and pipelines):

```bash
$ gpterminal vibe -y "list all docker containers"
```

When stdout is not a TTY (e.g. piped), vibe prints only the raw command without executing.

### Interactive chat

```bash
$ gpterminal chat
$ gpterminal chat "summarize this error"
$ cat server.log | gpterminal chat "what are the main failures?"
$ gpterminal chat --session bugfix
$ gpterminal resume bugfix
```

With no arguments, `chat` opens the full-screen TUI with markdown rendering and system context awareness. With a prompt and/or piped stdin, it runs in one-shot mode so you can use it in shell pipelines. Use `--session <name>` to save and resume a named conversation later with `gpterminal resume <name>`.

The chat assistant can also use local tools during a conversation to inspect files, list directories, search text, search the web, fetch web pages, stream responses, show thinking/tool status, run workspace commands directly from chat, and propose file writes with diff approval. Direct command execution in chat includes risk evaluation plus `yes` / `auto` / `no` approval, with auto-approve only available at or below a `7/10` risk threshold. File and path access is limited to the current working directory.

The chat TUI now shows the active session in the header and supports:
- `Ctrl+S` to save the current conversation into a named session
- `Ctrl+N` to start a fresh chat
- `Ctrl+X` to cancel the current AI response without closing the chat
- typing `exit` to quit

### GPTRun

```bash
$ gpterminal run "run the Go test suite"
```

`gptrun` generates one concrete command, shows you the command and its risk, lets you approve or edit it, then executes it. If the command fails, it can ask the AI for one retry command. Use `--yes`/`-y` to auto-approve execution:

```bash
$ gpterminal run -y "run the Go test suite"
```

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
$ gpterminal do "create a script called deploy.sh and make it executable"   # alias
$ gpterminal gptdo --yes "..."                                              # auto-approve non-denied
$ gpterminal gptdo --json --yes "..."                                       # machine-readable run report
$ gpterminal gptdo --session deploy-plan "..."
$ gpterminal resume deploy-plan
```

`gptdo` (alias `do`) asks the AI for a short ordered list of commands and runs them one at a time. **Authorization is decided by a local, deterministic policy — never by the AI.** Each command is classified as `allowed`, `needs_confirm`, or `denied`:

- `denied` commands (e.g. `rm -rf /`, `curl … | sh`, writing to a block device) are **refused and never executed** — `--yes` cannot override this.
- `needs_confirm` commands prompt before running (fail-closed: default is *no*).
- `allowed` commands run after a simple confirmation, with an `[a]uto` option.

`--yes`/`-y` auto-approves `allowed` and `needs_confirm` commands (handy in scripts) but still **cannot run a `denied` command**. An AI-provided risk score may also be shown, but it is advisory only. Command output is sent back to the AI so it can continue. Sessions are saved with `--session` (resume later), and rollback hints are shown when the AI proposes a practical undo.

`--json` emits a stable, machine-readable run report to stdout (human output goes to stderr), for CI/automation. It is non-interactive: `allowed` commands run, `needs_confirm` requires `--yes`, `denied` never runs. Each command records its `decision`, `reasons`, whether it `ran`, the `exit_code` (omitted when not run), and redacted `output`:

```json
{
  "schema_version": 1,
  "request": "create deploy.sh and make it executable",
  "cwd": "/repo",
  "steps": [
    { "index": 1, "proposed_commands": ["touch deploy.sh", "chmod +x deploy.sh"],
      "commands": [
        { "command": "touch deploy.sh", "decision": "allowed", "ran": true, "exit_code": 0 },
        { "command": "chmod +x deploy.sh", "decision": "allowed", "ran": true, "exit_code": 0 }
      ] }
  ],
  "completed": true,
  "aborted": false
}
```

### Doctor

```bash
$ gpterminal doctor
$ gpterminal doctor --json
```

`doctor` runs offline health checks and reports the configuration and **security posture**: active provider and whether its credential is set (the value is never printed), model, config file, whether the central execution policy and secret redaction are enabled, the MCP allowlist status, and tool availability. It exits non-zero if a required check fails, and `--json` emits a stable, redacted document for scripting.

### Eval

```bash
$ gpterminal eval
$ gpterminal eval --json
$ gpterminal eval --fixture cases.json
```

`eval` runs the deterministic command-safety policy against a fixture of commands and verifies each gets the expected decision (`allowed` / `needs_confirm` / `denied`). It uses no AI and no shell, exits non-zero on any mismatch, and doubles as a living spec / CI gate for the policy. The built-in suite covers dangerous patterns such as `rm -rf /`, `curl … | sh`, `cat .env`, and `chmod -R 777 .`. Supply your own cases with `--fixture` (a JSON array of `{name, command, expect}`).

### GPTSessions

```bash
$ gpterminal sessions
$ gpterminal sessions show bugfix
$ gpterminal sessions rename bugfix bugfix-v2
$ gpterminal sessions delete bugfix-v2
```

`gptsessions` lists saved chat and GPTDo sessions, shows details for one session, renames sessions, and deletes sessions you no longer need.

Use `--markdown` to export a session as a readable markdown document:

```bash
$ gpterminal sessions show bugfix --markdown > bugfix.md
```

### GPTS2T

```bash
$ gpterminal s2t ./meeting.wav
$ gpterminal s2t ./interview.m4a --translate
$ gpterminal s2t ./lecture.mp3 --format srt --output ./lecture.srt
$ gpterminal s2t --mic
```

`gpts2t` is available as a shell alias after running `gpterminal init <shell>`. The command uses OpenAI speech-to-text models to transcribe supported audio files (`mp3`, `mp4`, `mpeg`, `mpga`, `m4a`, `wav`, `webm`) up to 25 MB. Use `--translate` to translate speech into English instead of keeping the original language.

**Disclaimer:** You are solely responsible for the use of this transcription feature. Ensure you have proper authorization and consent before recording or transcribing any audio. Do not use this tool to transcribe conversations without the knowledge and explicit consent of all parties involved.

For live microphone transcription, use `gpterminal s2t --mic`. This streams 24 kHz mono PCM audio to OpenAI's Realtime API and prints incremental transcript updates until you press `Ctrl+C`. Use `--recorder` to force a backend and `--device` to target a specific source. Realtime mic mode currently supports text output only.

**Platform-specific mic backends:**

- **Linux**: prefers `pw-record`, then `parec`, then `arecord` (when ALSA exposes a real capture device), with `ffmpeg` (PulseAudio) as a final fallback
- **macOS**: prefers `sox` (`brew install sox`), then `ffmpeg` (AVFoundation)
- **Windows**: uses `ffmpeg` (DirectShow). You may need to specify `--device "Your Microphone Name"`

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
$ gpterminal abort deploy-plan          # delete the saved session
$ gpterminal abort deploy-plan --yes    # skip the confirmation prompt
```

`gpterminal resume <session>` resumes a named chat or GPTDo session. Use `--export` to print the saved session JSON. `gpterminal abort <session>` permanently removes a saved session's durable state so it can no longer be resumed (confirmation required unless `--yes`); it only deletes GPTerminal's own saved state and never executes commands or touches your files.

### GPTImagine

```bash
$ gpterminal imagine "a retro-futuristic terminal cockpit at sunrise"
$ gpterminal imagine "minimal icon set for a CLI tool" --n 3 --size 1024x1024 --output ./artifacts
```

`gptimagine` generates images with OpenAI image models and saves them to disk. You can choose the model, image size, image count, and output directory with flags.

### Inline Suggest

```bash
$ gpterminal suggest "git staus"
git status
```

Press `Ctrl+G` in your shell to trigger inline AI suggestion on the current command line buffer. The keybinding replaces your input with the completed or corrected command. Also available as the `gptsuggest` alias.

### Agent Mode

```bash
$ gpterminal agent "find all TODO comments in the codebase and list them"
$ gpterminal agent "create a hello world Python script" --session myagent
$ gpterminal agent "refactor the logging module" --max-steps 30
$ gpterminal resume myagent
```

`gptagent` launches an autonomous AI agent that plans and executes multi-step tasks. The agent uses available tools (read_file, list_directory, search_text, run_command, write_file, edit_file) to accomplish objectives. Commands with risk score above 7/10 require manual approval. Use `--session` to save progress and resume later. When MCP servers are configured, their tools are also available to the agent.

### Translate

```bash
$ gpterminal translate main.py go
$ gpterminal translate server.js typescript
$ gpterminal translate utils.go rust --output utils.rs
```

`gpttranslate` translates source code from one programming language to another. The AI auto-detects the source language, produces idiomatic code in the target language (not line-by-line transliteration), and suggests an output filename. You can override the output path with `--output`. Supports Go, Python, JavaScript, TypeScript, Rust, Ruby, Java, C, C++, C#, Swift, Kotlin, PHP, Lua, Scala, Bash, Zig, Elixir, Haskell, OCaml, R, Dart, and more.

### AI Shell

```bash
$ gpterminal shell
```

`gptshell` launches an AI-enhanced interactive shell that wraps your real shell. It shows a git-aware prompt and adds AI superpowers:

| Prefix | Action |
|--------|--------|
| `?`    | Ask AI a question (e.g., `? how do I find large files`) |
| `!`    | Generate a command from natural language (e.g., `! list docker containers by size`) |
| *(none)* | Run normally — failed commands trigger automatic AI fix suggestions |

Generated commands show a risk score before execution and support yes/no/edit responses. The shell tracks recent command history to give the AI context for better suggestions.

### GPTCode

```bash
$ gpterminal code
$ gpterminal code --session myproject
$ gpterminal resume myproject
```

`gptcode` launches an interactive AI coding assistant inspired by Claude Code. It auto-detects your project context (git branch, file tree, manifest files) and provides a persistent REPL for building, debugging, and maintaining software.

**Slash commands:**

| Command | Description |
|---------|-------------|
| `/help` | Show available commands |
| `/clear` | Clear conversation and start fresh |
| `/compact` | Summarize conversation to reduce context |
| `/diff` | Show git diff of project changes |
| `/status` | Show git status |
| `/undo` | Discard unstaged changes (with confirmation) |
| `/quit` | Exit GPTCode |

The assistant has full tool access (read_file, edit_file, write_file, run_command, search_text, web_search, fetch_url) with approval workflows for file writes and commands. Use `--session` to save and resume coding sessions.

### Conversation Memory

```bash
$ gpterminal memory list              # List all saved memories
$ gpterminal memory set lang Go       # Save a memory
$ gpterminal memory delete lang       # Delete a memory
$ gpterminal memory search project    # Search memories
$ gpterminal memory clear             # Clear all memories
```

The AI can also save and delete memories automatically during chat and agent sessions using the `save_memory` and `delete_memory` tools. Saved memories are injected into future conversations so the AI remembers your project context, preferences, and key details across sessions.

### Stats Dashboard

```bash
$ gpterminal stats
$ gpterminal stats --tui
```

`gptstats` shows usage statistics including total cost, API calls, tokens used, per-command breakdowns, and daily cost trends. Use `--tui` for an interactive terminal dashboard.

### Auto-Update

```bash
$ gpterminal update --check    # Check for updates without installing
$ gpterminal update            # Download and install the latest version
```

Downloads pre-built binaries from GitHub Releases and replaces the current binary atomically.

### Custom Templates

```bash
$ gpterminal template create explain    # Create a starter template
$ gpterminal template list              # List available templates
$ gpterminal explain "recursion"        # Use a custom template command
```

Define custom AI commands via YAML files in `~/.config/gpterminal/templates/`. Each template becomes a top-level command with configurable system prompt, input mode, variables, and streaming options.

Example template (`~/.config/gpterminal/templates/explain.yaml`):

```yaml
name: explain
description: "Explain a concept simply"
system_prompt: |
  You are a patient teacher. Explain the following concept.
  Target audience: {{audience}}.
input_mode: args
variables:
  audience: "developers"
stream: true
use_tools: false
```

### MCP Support

GPTerminal supports the Model Context Protocol (MCP) for extending available tools. Configure MCP servers in `~/.config/gpterminal/config.yaml`:

```yaml
mcp_servers:
  filesystem:
    command: npx
    args: ["-y", "@modelcontextprotocol/server-filesystem", "/home"]
  github:
    command: npx
    args: ["-y", "@modelcontextprotocol/server-github"]
    env:
      GITHUB_TOKEN: "ghp_..."
```

MCP tools are automatically available in `chat` and `agent` commands. If an MCP server crashes mid-session, GPTerminal will automatically reconnect and retry the failed call.

### Command Hooks

Configure shell commands that run before or after command execution in `~/.config/gpterminal/config.yaml`:

```yaml
hooks:
  pre_command:
    - command: "echo 'running: $GPT_COMMAND'"
  post_command:
    - command: "echo 'done: exit $GPT_EXIT_CODE'"
```

Available environment variables: `GPT_COMMAND`, `GPT_WORK_DIR`, `GPT_EXIT_CODE` (post only), `GPT_EVENT`.

### Enhanced Pipe Mode

All commands now support stdin piping:

```bash
$ git diff | gpterminal review           # Review piped diff
$ git diff | gpterminal explain-diff     # Explain piped diff
$ echo "rm -rf /" | gpterminal risk      # Evaluate piped command risk
$ echo "list files" | gpterminal vibe    # Pipe description to vibe
$ gpterminal risk "rm -rf /" | cat       # Plain output when piped
```

Colors and spinners are automatically stripped when stdout is not a TTY.

### Security & Safety

GPTerminal treats the AI as an untrusted advisor: the AI may *propose* a command, but only a **local, deterministic policy** may *authorize* running it. Commands are parsed (not just string-matched) and classified as `allowed`, `needs_confirm`, or `denied`. A `denied` command — e.g. `rm -rf /`, `curl … | sh`, writing to a block device, fork bombs — is never executed, and `--yes` cannot override it. Unknown or ambiguous input fails closed (confirmation required).

Additional safeguards:

- **Secret redaction** — API keys, tokens, JWTs, private-key blocks, and secret-named values are masked before any text is sent to an AI provider (and in logs/JSON output).
- **MCP allowlist** — when configured, only approved MCP server commands may launch.
- **Read-only `review`** — `gpterminal review` never writes to disk, mutates git, or runs commands.

Environment toggles (all default to the safe setting):

| Variable | Default | Effect when set to `0`/`off` |
| --- | --- | --- |
| `GPTERMINAL_EXEC_POLICY` | on | Disables the central command policy (commands bypass classification) |
| `GPTERMINAL_REDACT` | on | Disables secret redaction before AI requests |
| `GPTERMINAL_MCP_ALLOWLIST` | unset | When set (comma-separated commands), restricts which MCP servers may start |

Run `gpterminal doctor` to see the current security posture, and `gpterminal eval` to verify the policy.

### Shell Completions

Generate tab-completion scripts for your shell:

```bash
# Bash
gpterminal completion bash > /etc/bash_completion.d/gpterminal

# Zsh
gpterminal completion zsh > "${fpath[1]}/_gpterminal"

# Fish
gpterminal completion fish > ~/.config/fish/completions/gpterminal.fish

# PowerShell
gpterminal completion powershell | Out-String | Invoke-Expression
```

### Cost Tracking

View daily or weekly API cost breakdowns:

```bash
$ gpterminal config usage           # monthly summary
$ gpterminal config usage --daily   # per-day breakdown
$ gpterminal config usage --weekly  # per-week breakdown
```

### Configuration

```bash
gpterminal config set-provider <provider>    # Set provider (openai, anthropic, gemini, openclaw)
gpterminal config set-key <key>              # Save OpenAI API key
gpterminal config set-anthropic-key <key>    # Save Anthropic API key
gpterminal config set-gemini-key <key>       # Save Gemini API key
gpterminal config set-base-url <url>         # Save API base URL (e.g. Ollama)
gpterminal config show                       # Show current config (with validation warnings)
```

Config is stored at `~/.config/gpterminal/config.yaml`.

## Configuration Options

| Key | Default | Env Variable | Description |
|-----|---------|-------------|-------------|
| `provider` | `openai` | `GPTERMINAL_PROVIDER` | AI provider: `openai`, `anthropic`, `gemini`, or `openclaw` |
| `api_key` | - | `OPENAI_API_KEY` | OpenAI API key |
| `anthropic_api_key` | - | `ANTHROPIC_API_KEY` | Anthropic API key |
| `gemini_api_key` | - | `GEMINI_API_KEY` | Google Gemini API key |
| `api_base_url` | `https://api.openai.com/v1` | `OPENAI_API_BASE_URL` | API base URL (Ollama, etc.) |
| `model` | `gpt-4o-mini` | `OPENAI_MODEL` | Model to use |
| `temperature` | `0.7` | - | Response creativity |
| `max_tokens` | `2048` | - | Max response length |
| `openclaw_url` | - | `OPENCLAW_URL` | OpenClaw Gateway URL |
| `openclaw_token` | - | `OPENCLAW_TOKEN` | OpenClaw Bearer token |
| `openclaw_agent` | - | `OPENCLAW_AGENT` | OpenClaw agent name |
| `openclaw_password` | - | `OPENCLAW_PASSWORD` | OpenClaw shared password (Bearer auth fallback) |
| `mcp_servers` | - | - | MCP server configurations (see MCP Support) |

## Author

Made by [@Cycl0o0](https://github.com/cycl0o0)

## License

This project is licensed under the GNU Affero General Public License v3.0 - see the [LICENSE](LICENSE) file for details.
