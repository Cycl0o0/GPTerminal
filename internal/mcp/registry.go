package mcp

import (
	"encoding/json"
	"fmt"
	"strings"

	openai "github.com/sashabaranov/go-openai"
	"github.com/spf13/viper"
)

type registeredTool struct {
	client *Client
	def    ToolDef
}

type Registry struct {
	clients map[string]*Client
	tools   map[string]*registeredTool
}

func NewRegistry() *Registry {
	return &Registry{
		clients: make(map[string]*Client),
		tools:   make(map[string]*registeredTool),
	}
}

func (r *Registry) LoadFromConfig() error {
	servers := viper.GetStringMap("mcp_servers")
	if len(servers) == 0 {
		return nil
	}

	for name := range servers {
		raw := viper.GetStringMap("mcp_servers." + name)
		cfg := ServerConfig{}

		if cmd, ok := raw["command"]; ok {
			cfg.Command = fmt.Sprint(cmd)
		}
		if args, ok := raw["args"]; ok {
			if argsList, ok := args.([]interface{}); ok {
				for _, a := range argsList {
					cfg.Args = append(cfg.Args, fmt.Sprint(a))
				}
			}
		}
		if env, ok := raw["env"]; ok {
			if envMap, ok := env.(map[string]interface{}); ok {
				cfg.Env = make(map[string]string)
				for k, v := range envMap {
					cfg.Env[k] = fmt.Sprint(v)
				}
			}
		}

		if cfg.Command == "" {
			fmt.Printf("Warning: MCP server %q has no command, skipping\n", name)
			continue
		}

		client := NewClient(name, cfg)
		if err := client.Start(); err != nil {
			return fmt.Errorf("start MCP server %q: %w", name, err)
		}
		r.clients[name] = client

		tools, err := client.ListTools()
		if err != nil {
			client.Close()
			return fmt.Errorf("list tools from %q: %w", name, err)
		}

		for _, tool := range tools {
			toolName := tool.Name
			// Handle name collisions by prefixing
			if _, exists := r.tools[toolName]; exists {
				toolName = name + "__" + toolName
			}
			r.tools[toolName] = &registeredTool{client: client, def: tool}
		}
	}

	return nil
}

func (r *Registry) Tools() []openai.Tool {
	var tools []openai.Tool
	for name, rt := range r.tools {
		// Convert the JSON schema to the format expected by openai.Tool
		params := rt.def.InputSchema
		if params == nil {
			params = map[string]interface{}{
				"type":       "object",
				"properties": map[string]interface{}{},
			}
		}

		tools = append(tools, openai.Tool{
			Type: openai.ToolTypeFunction,
			Function: &openai.FunctionDefinition{
				Name:        name,
				Description: rt.def.Description,
				Parameters:  params,
			},
		})
	}
	return tools
}

func (r *Registry) HandleToolCall(name, arguments string) (string, error) {
	rt, ok := r.tools[name]
	if !ok {
		return "", fmt.Errorf("unknown MCP tool: %s", name)
	}

	// Parse arguments
	var args map[string]interface{}
	if arguments != "" {
		if err := json.Unmarshal([]byte(arguments), &args); err != nil {
			return "", fmt.Errorf("parse MCP tool arguments: %w", err)
		}
	}

	// Use the original tool name (without prefix) for the actual call
	originalName := rt.def.Name
	result, err := rt.client.CallTool(originalName, args)
	if err != nil {
		return "Error: " + err.Error(), nil
	}
	return result, nil
}

func (r *Registry) HasTool(name string) bool {
	_, ok := r.tools[name]
	return ok
}

func (r *Registry) Close() {
	for _, client := range r.clients {
		client.Close()
	}
}

func (r *Registry) Summary() string {
	if len(r.tools) == 0 {
		return "no MCP tools loaded"
	}
	var names []string
	for name := range r.tools {
		names = append(names, name)
	}
	return fmt.Sprintf("%d MCP tools: %s", len(names), strings.Join(names, ", "))
}
