package mcp

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"sync"
)

type ServerConfig struct {
	Command string            `yaml:"command" json:"command"`
	Args    []string          `yaml:"args"    json:"args"`
	Env     map[string]string `yaml:"env"     json:"env,omitempty"`
}

type Client struct {
	name   string
	config ServerConfig
	cmd    *exec.Cmd
	stdin  io.WriteCloser
	stdout *bufio.Reader
	nextID int
	mu     sync.Mutex
}

func NewClient(name string, cfg ServerConfig) *Client {
	return &Client{
		name:   name,
		config: cfg,
		nextID: 1,
	}
}

func (c *Client) Start() error {
	c.cmd = exec.Command(c.config.Command, c.config.Args...)
	c.cmd.Stderr = os.Stderr

	// Set environment
	c.cmd.Env = os.Environ()
	for k, v := range c.config.Env {
		c.cmd.Env = append(c.cmd.Env, k+"="+v)
	}

	var err error
	c.stdin, err = c.cmd.StdinPipe()
	if err != nil {
		return fmt.Errorf("mcp %s: stdin pipe: %w", c.name, err)
	}

	stdout, err := c.cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("mcp %s: stdout pipe: %w", c.name, err)
	}
	c.stdout = bufio.NewReader(stdout)

	if err := c.cmd.Start(); err != nil {
		return fmt.Errorf("mcp %s: start: %w", c.name, err)
	}

	// Send initialize
	resp, err := c.call("initialize", map[string]interface{}{
		"protocolVersion": "2024-11-05",
		"capabilities":    map[string]interface{}{},
		"clientInfo": map[string]interface{}{
			"name":    "gpterminal",
			"version": "2.2.0",
		},
	})
	if err != nil {
		c.Close()
		return fmt.Errorf("mcp %s: initialize: %w", c.name, err)
	}
	_ = resp // We don't need to parse the result

	// Send initialized notification
	c.notify("notifications/initialized", nil)

	return nil
}

func (c *Client) ListTools() ([]ToolDef, error) {
	resp, err := c.call("tools/list", nil)
	if err != nil {
		return nil, err
	}

	var result toolsListResult
	if err := json.Unmarshal(resp, &result); err != nil {
		return nil, fmt.Errorf("mcp %s: parse tools: %w", c.name, err)
	}
	return result.Tools, nil
}

func (c *Client) CallTool(name string, args map[string]interface{}) (string, error) {
	resp, err := c.call("tools/call", map[string]interface{}{
		"name":      name,
		"arguments": args,
	})
	if err != nil {
		return "", err
	}

	var result CallResult
	if err := json.Unmarshal(resp, &result); err != nil {
		return "", fmt.Errorf("mcp %s: parse tool result: %w", c.name, err)
	}

	var parts []string
	for _, block := range result.Content {
		if block.Type == "text" {
			parts = append(parts, block.Text)
		}
	}
	text := strings.Join(parts, "\n")

	if result.IsError {
		return "", fmt.Errorf("mcp tool error: %s", text)
	}
	return text, nil
}

func (c *Client) Close() error {
	if c.stdin != nil {
		c.stdin.Close()
	}
	if c.cmd != nil && c.cmd.Process != nil {
		c.cmd.Process.Kill()
		c.cmd.Wait()
	}
	return nil
}

func (c *Client) call(method string, params interface{}) (json.RawMessage, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	id := c.nextID
	c.nextID++

	req := Request{
		JSONRPC: "2.0",
		ID:      id,
		Method:  method,
		Params:  params,
	}

	data, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}

	if _, err := c.stdin.Write(append(data, '\n')); err != nil {
		return nil, fmt.Errorf("mcp %s: write: %w", c.name, err)
	}

	// Read response line
	line, err := c.stdout.ReadBytes('\n')
	if err != nil {
		return nil, fmt.Errorf("mcp %s: read: %w", c.name, err)
	}

	var resp Response
	if err := json.Unmarshal(line, &resp); err != nil {
		return nil, fmt.Errorf("mcp %s: parse response: %w", c.name, err)
	}

	if resp.Error != nil {
		return nil, resp.Error
	}

	return resp.Result, nil
}

func (c *Client) notify(method string, params interface{}) {
	req := Request{
		JSONRPC: "2.0",
		Method:  method,
		Params:  params,
	}
	data, _ := json.Marshal(req)
	c.stdin.Write(append(data, '\n'))
}
