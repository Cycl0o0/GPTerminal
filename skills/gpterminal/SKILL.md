# GPTerminal — OpenClaw Skill

GPTerminal is an AI-powered terminal assistant. When deployed as an OpenClaw
skill, it exposes CLI subcommands that the agent can invoke to interact with
the user's local environment.

## Available Commands

| Command | Description |
|---------|-------------|
| `gpterminal chat "<prompt>"` | One-shot AI chat with optional piped stdin |
| `gpterminal do "<instruction>"` | Execute a natural-language command |
| `gpterminal fix` | Fix the last failed shell command |
| `gpterminal review [path]` | AI code review of a file or git diff |
| `gpterminal risk "<command>"` | Risk-score a shell command before running it |
| `gpterminal commit` | Generate a conventional commit message from staged changes |
| `gpterminal code` | Interactive coding assistant (GPTCode) |
| `gpterminal vibe "<prompt>"` | Autonomous agent — plans and executes multi-step tasks |
| `gpterminal suggest` | Suggest the next command based on shell history |

## Usage Notes

- All commands respect the current working directory.
- `chat` and `do` accept piped stdin: `cat file | gpterminal chat "summarize"`.
- `review` with no arguments reviews the current git diff.
- `vibe` runs autonomously and may execute commands — use with caution.
- Configuration is stored in `~/.config/gpterminal/config.yaml`.

## Deployment

Copy this directory (`skills/gpterminal/`) to your OpenClaw server's skills
directory, or reference it from your agent configuration. Ensure `gpterminal`
is installed and accessible in `$PATH` on the machine where the skill runs.
