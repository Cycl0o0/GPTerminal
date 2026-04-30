package hooks

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"sync"

	"github.com/spf13/viper"
)

// Event represents a hook trigger point.
type Event string

const (
	PreCommand  Event = "pre_command"
	PostCommand Event = "post_command"
	PreToolCall Event = "pre_tool"
	PostToolCall Event = "post_tool"
)

// CommandContext holds data passed to pre-command hooks.
type CommandContext struct {
	Command string
	Args    []string
	WorkDir string
}

// CommandResult holds data passed to post-command hooks.
type CommandResult struct {
	Command  string
	ExitCode int
	Output   string
	Err      error
}

// HookFunc is a user-registered in-process hook callback.
type HookFunc func(ctx context.Context, data interface{}) error

// HookConfig represents a configured shell hook from config.yaml.
type HookConfig struct {
	Command string `yaml:"command" mapstructure:"command"`
}

// Registry manages hook subscriptions and fires events.
type Registry struct {
	mu    sync.RWMutex
	hooks map[Event][]HookFunc
	shell map[Event][]HookConfig
}

// NewRegistry creates a Registry and loads shell hooks from config.
func NewRegistry() *Registry {
	r := &Registry{
		hooks: make(map[Event][]HookFunc),
		shell: make(map[Event][]HookConfig),
	}
	r.loadFromConfig()
	return r
}

func (r *Registry) loadFromConfig() {
	raw := viper.GetStringMap("hooks")
	if len(raw) == 0 {
		return
	}
	for _, event := range []Event{PreCommand, PostCommand, PreToolCall, PostToolCall} {
		entries := viper.Get("hooks." + string(event))
		if entries == nil {
			continue
		}
		list, ok := entries.([]interface{})
		if !ok {
			continue
		}
		for _, entry := range list {
			m, ok := entry.(map[string]interface{})
			if !ok {
				continue
			}
			cmd, ok := m["command"]
			if !ok {
				continue
			}
			r.shell[event] = append(r.shell[event], HookConfig{Command: fmt.Sprint(cmd)})
		}
	}
}

// On registers an in-process hook function for an event.
func (r *Registry) On(event Event, fn HookFunc) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.hooks[event] = append(r.hooks[event], fn)
}

// Fire triggers all hooks for the given event.
func (r *Registry) Fire(ctx context.Context, event Event, data interface{}) error {
	r.mu.RLock()
	fns := r.hooks[event]
	shells := r.shell[event]
	r.mu.RUnlock()

	// Run in-process hooks.
	for _, fn := range fns {
		if err := fn(ctx, data); err != nil {
			return err
		}
	}

	// Run shell hooks.
	for _, hc := range shells {
		if err := r.runShellHook(ctx, event, hc, data); err != nil {
			fmt.Fprintf(os.Stderr, "hook %s error: %v\n", event, err)
		}
	}

	return nil
}

func (r *Registry) runShellHook(ctx context.Context, event Event, hc HookConfig, data interface{}) error {
	cmd := exec.CommandContext(ctx, "sh", "-c", hc.Command)
	cmd.Stderr = os.Stderr
	cmd.Stdout = os.Stderr

	env := os.Environ()
	env = append(env, "GPT_EVENT="+string(event))

	switch v := data.(type) {
	case *CommandContext:
		env = append(env, "GPT_COMMAND="+v.Command)
		env = append(env, "GPT_WORK_DIR="+v.WorkDir)
	case *CommandResult:
		env = append(env, "GPT_COMMAND="+v.Command)
		env = append(env, "GPT_EXIT_CODE="+strconv.Itoa(v.ExitCode))
	}

	cmd.Env = env
	return cmd.Run()
}
