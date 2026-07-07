package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/spf13/viper"
)

const (
	DefaultModel                = "gpt-4o-mini"
	DefaultTemp                 = 0.7
	DefaultMaxTokens            = 8192
	DefaultCostLimit            = 0.0 // 0 = unlimited
	DefaultWarnPct              = 80.0
	DefaultImageModel           = "gpt-image-1"
	DefaultImageSize            = "1024x1024"
	DefaultBaseURL              = "https://api.openai.com/v1"
	DefaultS2TModel             = "gpt-4o-mini-transcribe"
	DefaultT2SModel             = "gpt-4o-mini-tts"
	DefaultT2SVoice             = "marin"
	DefaultRealtimeSessionModel = "gpt-realtime"

	// Agentic / TUI settings (v3.3).
	DefaultApprovalMode = "default" // plan | default | auto-edit | yolo
	DefaultEffort       = "none"    // none | minimal | low | medium | high | max
	DefaultTheme        = "auto"    // auto | dark | light
	DefaultTUIMode      = "auto"    // auto | on | off
	DefaultMaxToolRounds = 0        // 0 = engine default (200)
	DefaultAutoCompact  = false
)

func Init() {
	viper.SetConfigName("config")
	viper.SetConfigType("yaml")

	configDir := ConfigDir()
	viper.AddConfigPath(configDir)

	viper.SetDefault("provider", "openai")
	viper.SetDefault("model", DefaultModel)
	viper.SetDefault("temperature", DefaultTemp)
	viper.SetDefault("max_tokens", DefaultMaxTokens)
	viper.SetDefault("cost_limit", DefaultCostLimit)
	viper.SetDefault("warn_threshold", DefaultWarnPct)
	viper.SetDefault("image_model", DefaultImageModel)
	viper.SetDefault("image_size", DefaultImageSize)
	viper.SetDefault("api_base_url", DefaultBaseURL)
	viper.SetDefault("s2t_model", DefaultS2TModel)
	viper.SetDefault("t2s_model", DefaultT2SModel)
	viper.SetDefault("t2s_voice", DefaultT2SVoice)
	viper.SetDefault("approval_mode", DefaultApprovalMode)
	viper.SetDefault("effort", DefaultEffort)
	viper.SetDefault("theme", DefaultTheme)
	viper.SetDefault("tui", DefaultTUIMode)
	viper.SetDefault("max_tool_rounds", DefaultMaxToolRounds)
	viper.SetDefault("auto_compact", DefaultAutoCompact)

	viper.SetEnvPrefix("OPENAI")
	viper.BindEnv("api_key")
	viper.BindEnv("model", "OPENAI_MODEL")
	viper.BindEnv("api_base_url", "OPENAI_API_BASE_URL")

	viper.BindEnv("anthropic_api_key", "ANTHROPIC_API_KEY")
	viper.BindEnv("gemini_api_key", "GEMINI_API_KEY")
	viper.BindEnv("provider", "GPTERMINAL_PROVIDER")

	viper.BindEnv("openclaw_url", "OPENCLAW_URL")
	viper.BindEnv("openclaw_token", "OPENCLAW_TOKEN")
	viper.BindEnv("openclaw_agent", "OPENCLAW_AGENT")
	viper.BindEnv("openclaw_password", "OPENCLAW_PASSWORD")

	viper.BindEnv("approval_mode", "GPTERMINAL_APPROVAL_MODE")
	viper.BindEnv("effort", "GPTERMINAL_EFFORT")
	viper.BindEnv("theme", "GPTERMINAL_THEME")
	viper.BindEnv("tui", "GPTERMINAL_TUI")

	_ = viper.ReadInConfig()
}

func ConfigDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		home = "."
	}
	dir := filepath.Join(home, ".config", "gpterminal")
	return dir
}

func ConfigFile() string {
	return filepath.Join(ConfigDir(), "config.yaml")
}

func APIKey() string {
	return viper.GetString("api_key")
}

func Model() string {
	return viper.GetString("model")
}

func Temperature() float32 {
	return float32(viper.GetFloat64("temperature"))
}

func MaxTokens() int {
	return viper.GetInt("max_tokens")
}

func CostLimit() float64 {
	return viper.GetFloat64("cost_limit")
}

func WarnThreshold() float64 {
	return viper.GetFloat64("warn_threshold")
}

func ImageModel() string {
	return viper.GetString("image_model")
}

func ImageSize() string {
	return viper.GetString("image_size")
}

func APIBaseURL() string {
	return viper.GetString("api_base_url")
}

func S2TModel() string {
	return viper.GetString("s2t_model")
}

func T2SModel() string {
	return viper.GetString("t2s_model")
}

func T2SVoice() string {
	return viper.GetString("t2s_voice")
}

// S2TBaseURL returns the base URL for speech-to-text requests.
// Falls back to the main api_base_url if not set.
func S2TBaseURL() string {
	if u := viper.GetString("s2t_base_url"); u != "" {
		return u
	}
	return APIBaseURL()
}

// T2SBaseURL returns the base URL for text-to-speech requests.
// Falls back to the main api_base_url if not set.
func T2SBaseURL() string {
	if u := viper.GetString("t2s_base_url"); u != "" {
		return u
	}
	return APIBaseURL()
}

// ImageBaseURL returns the base URL for image generation requests.
// Falls back to the main api_base_url if not set.
func ImageBaseURL() string {
	if u := viper.GetString("image_base_url"); u != "" {
		return u
	}
	return APIBaseURL()
}

// RealtimeURL returns the WebSocket URL for realtime transcription.
// Derived from api_base_url when not explicitly set (https→wss, http→ws).
func RealtimeURL() string {
	if u := viper.GetString("realtime_url"); u != "" {
		return u
	}
	base := strings.TrimRight(APIBaseURL(), "/")
	base = strings.TrimSuffix(base, "/realtime")
	switch {
	case strings.HasPrefix(base, "https://"):
		return strings.Replace(base, "https://", "wss://", 1) + "/realtime"
	case strings.HasPrefix(base, "http://"):
		return strings.Replace(base, "http://", "ws://", 1) + "/realtime"
	}
	return base + "/realtime"
}

// RealtimeModel returns the configured realtime session model.
func RealtimeModel() string {
	if m := viper.GetString("realtime_model"); m != "" {
		return m
	}
	return DefaultRealtimeSessionModel
}

func ProviderName() string {
	p := strings.ToLower(viper.GetString("provider"))
	if p == "" {
		return "openai"
	}
	return p
}

func AnthropicAPIKey() string {
	return viper.GetString("anthropic_api_key")
}

func GeminiAPIKey() string {
	return viper.GetString("gemini_api_key")
}

func OpenClawURL() string      { return viper.GetString("openclaw_url") }
func OpenClawToken() string    { return viper.GetString("openclaw_token") }
func OpenClawAgent() string    { return viper.GetString("openclaw_agent") }
func OpenClawPassword() string { return viper.GetString("openclaw_password") }

func MCPServers() map[string]interface{} {
	return viper.GetStringMap("mcp_servers")
}

// --- Agentic / TUI settings (v3.3) ---

// ApprovalMode returns the configured approval policy: plan | default |
// auto-edit | yolo. Unknown values fall back to "default".
func ApprovalMode() string {
	m := strings.ToLower(strings.TrimSpace(viper.GetString("approval_mode")))
	switch m {
	case "plan", "default", "auto-edit", "yolo":
		return m
	default:
		return DefaultApprovalMode
	}
}

// Effort returns the configured reasoning effort: none | minimal | low |
// medium | high | max. Unknown values fall back to "none".
func Effort() string {
	e := strings.ToLower(strings.TrimSpace(viper.GetString("effort")))
	switch e {
	case "none", "minimal", "low", "medium", "high", "max":
		return e
	default:
		return DefaultEffort
	}
}

// Theme returns the UI theme preference: auto | dark | light.
func Theme() string {
	t := strings.ToLower(strings.TrimSpace(viper.GetString("theme")))
	switch t {
	case "auto", "dark", "light":
		return t
	default:
		return DefaultTheme
	}
}

// TUIMode returns whether the full-screen TUI should be used: auto | on | off.
func TUIMode() string {
	t := strings.ToLower(strings.TrimSpace(viper.GetString("tui")))
	switch t {
	case "auto", "on", "off":
		return t
	default:
		return DefaultTUIMode
	}
}

// MaxToolRounds returns the configured per-turn tool-round cap; 0 means "use
// the engine default".
func MaxToolRounds() int {
	n := viper.GetInt("max_tool_rounds")
	if n < 0 {
		return 0
	}
	return n
}

// AutoCompact reports whether long sessions should be auto-summarized.
func AutoCompact() bool {
	return viper.GetBool("auto_compact")
}

// saveValue is a shared helper used by all string Save* functions.
func saveValue(key, value string) error {
	return saveAny(key, value)
}

// saveAny sets any value type (string/int/bool) and persists the config.
//
// It writes through a SEPARATE viper instance seeded only from the on-disk
// file, not the global viper. This is deliberate: the global viper carries
// process-only overrides (SetActiveModel/Effort/Provider) and env-derived
// secrets in its override/env layers, and viper.WriteConfigAs serializes the
// merged AllSettings() — so writing through the global viper would silently
// persist a temporary /model override or an env API key to disk. Round-tripping
// through a file-only instance keeps those out of the file while still updating
// the live global value so the change takes effect immediately.
func saveAny(key string, value any) error {
	dir := ConfigDir()
	if err := os.MkdirAll(dir, 0700); err != nil {
		return fmt.Errorf("create config dir: %w", err)
	}
	cfgFile := ConfigFile()

	disk := viper.New()
	disk.SetConfigFile(cfgFile)
	if err := disk.ReadInConfig(); err != nil && !os.IsNotExist(err) {
		// A missing file is fine (first write); other read errors (corrupt
		// YAML) must not be silently clobbered.
		if _, statErr := os.Stat(cfgFile); statErr == nil {
			return fmt.Errorf("read config before write: %w", err)
		}
	}
	disk.Set(key, value)
	if err := disk.WriteConfigAs(cfgFile); err != nil {
		return fmt.Errorf("write config: %w", err)
	}
	if err := os.Chmod(cfgFile, 0600); err != nil {
		return err
	}
	// Reflect the change in the live global viper so it takes effect now.
	viper.Set(key, value)
	return nil
}

// KeyKind classifies a settable config key for validation and display.
type KeyKind int

const (
	KindString KeyKind = iota
	KindInt
	KindBool
	KindFloat
	KindEnum
	KindSecret // string, but masked in listings
)

// KeyDef describes one user-settable config key. The registry drives the
// generic `config get/set/list` CLI and the settings screens so every key is
// defined in exactly one place.
type KeyDef struct {
	Key     string
	Kind    KeyKind
	Default string
	Desc    string
	Enum    []string // valid values when Kind == KindEnum
}

// SettableKeys is the registry of keys exposed via `config get/set/list`.
// Provider credentials keep their dedicated set-* commands but are listed here
// too (as secrets) so `config list` shows their state.
var SettableKeys = []KeyDef{
	{Key: "provider", Kind: KindEnum, Default: "openai", Desc: "AI provider", Enum: []string{"openai", "anthropic", "gemini", "openclaw"}},
	{Key: "model", Kind: KindString, Default: DefaultModel, Desc: "Chat model"},
	{Key: "temperature", Kind: KindFloat, Default: "0.7", Desc: "Sampling temperature (0.0-2.0)"},
	{Key: "max_tokens", Kind: KindInt, Default: fmt.Sprintf("%d", DefaultMaxTokens), Desc: "Max completion tokens"},
	{Key: "effort", Kind: KindEnum, Default: DefaultEffort, Desc: "Reasoning effort (provider-dependent)", Enum: []string{"none", "minimal", "low", "medium", "high", "max"}},
	{Key: "approval_mode", Kind: KindEnum, Default: DefaultApprovalMode, Desc: "Tool approval policy", Enum: []string{"plan", "default", "auto-edit", "yolo"}},
	{Key: "theme", Kind: KindEnum, Default: DefaultTheme, Desc: "UI theme", Enum: []string{"auto", "dark", "light"}},
	{Key: "tui", Kind: KindEnum, Default: DefaultTUIMode, Desc: "Use full-screen TUI for code mode", Enum: []string{"auto", "on", "off"}},
	{Key: "max_tool_rounds", Kind: KindInt, Default: "0", Desc: "Per-turn tool-round cap (0 = default 200)"},
	{Key: "auto_compact", Kind: KindBool, Default: "false", Desc: "Auto-summarize long sessions"},
	{Key: "cost_limit", Kind: KindFloat, Default: "0", Desc: "Monthly USD cost limit (0 = unlimited)"},
	{Key: "warn_threshold", Kind: KindFloat, Default: "80", Desc: "Warn at this percent of cost limit"},
	{Key: "api_base_url", Kind: KindString, Default: DefaultBaseURL, Desc: "OpenAI-compatible base URL"},
	{Key: "api_key", Kind: KindSecret, Desc: "OpenAI API key"},
	{Key: "anthropic_api_key", Kind: KindSecret, Desc: "Anthropic API key"},
	{Key: "gemini_api_key", Kind: KindSecret, Desc: "Gemini API key"},
	{Key: "image_model", Kind: KindString, Default: DefaultImageModel, Desc: "Image generation model"},
	{Key: "image_size", Kind: KindString, Default: DefaultImageSize, Desc: "Image size"},
}

// LookupKey returns the KeyDef for a key name, or false if not settable.
func LookupKey(key string) (KeyDef, bool) {
	key = strings.ToLower(strings.TrimSpace(key))
	for _, d := range SettableKeys {
		if d.Key == key {
			return d, true
		}
	}
	return KeyDef{}, false
}

// GetValue returns the current string form of any key's value.
func GetValue(key string) string {
	return viper.GetString(strings.ToLower(strings.TrimSpace(key)))
}

// SetValue validates value against the key's KeyDef and persists it. It is the
// backend for the generic `config set <key> <value>` command.
func SetValue(key, value string) error {
	def, ok := LookupKey(key)
	if !ok {
		return fmt.Errorf("unknown setting %q (run `gpterminal config list`)", key)
	}
	value = strings.TrimSpace(value)
	switch def.Kind {
	case KindInt:
		n, err := strconv.Atoi(value)
		if err != nil {
			return fmt.Errorf("%s must be an integer: %w", def.Key, err)
		}
		return saveAny(def.Key, n)
	case KindBool:
		b, err := strconv.ParseBool(value)
		if err != nil {
			return fmt.Errorf("%s must be true or false: %w", def.Key, err)
		}
		return saveAny(def.Key, b)
	case KindFloat:
		f, err := strconv.ParseFloat(value, 64)
		if err != nil {
			return fmt.Errorf("%s must be a number: %w", def.Key, err)
		}
		return saveAny(def.Key, f)
	case KindEnum:
		lower := strings.ToLower(value)
		for _, v := range def.Enum {
			if v == lower {
				return saveAny(def.Key, lower)
			}
		}
		return fmt.Errorf("%s must be one of: %s", def.Key, strings.Join(def.Enum, ", "))
	default:
		return saveAny(def.Key, value)
	}
}

func SaveProvider(provider string) error   { return saveValue("provider", provider) }
func SaveAnthropicAPIKey(key string) error { return saveValue("anthropic_api_key", key) }
func SaveGeminiAPIKey(key string) error    { return saveValue("gemini_api_key", key) }
func SaveAPIBaseURL(url string) error      { return saveValue("api_base_url", url) }
func SaveModel(model string) error         { return saveValue("model", model) }

// SetActiveModel overrides the active model for the current process only,
// without writing to disk. Subsequent config.Model() calls return this value.
func SetActiveModel(model string) {
	if strings.TrimSpace(model) != "" {
		viper.Set("model", strings.TrimSpace(model))
	}
}

// SetActiveEffort overrides the reasoning effort for the current process only
// (no disk write), mirroring SetActiveModel.
func SetActiveEffort(effort string) {
	if strings.TrimSpace(effort) != "" {
		viper.Set("effort", strings.TrimSpace(effort))
	}
}

// SetActiveProvider overrides the provider for the current process only (no
// disk write), used by serve mode's per-request provider selection.
func SetActiveProvider(provider string) {
	if strings.TrimSpace(provider) != "" {
		viper.Set("provider", strings.ToLower(strings.TrimSpace(provider)))
	}
}

// SaveApprovalMode / SaveEffort / SaveTheme persist agentic settings to disk.
func SaveApprovalMode(mode string) error { return SetValue("approval_mode", mode) }
func SaveEffort(effort string) error     { return SetValue("effort", effort) }
func SaveTheme(theme string) error       { return SetValue("theme", theme) }
func SaveAPIKey(key string) error            { return saveValue("api_key", key) }
func SaveS2TModel(model string) error        { return saveValue("s2t_model", model) }
func SaveT2SModel(model string) error        { return saveValue("t2s_model", model) }
func SaveT2SVoice(voice string) error        { return saveValue("t2s_voice", voice) }
func SaveImageModel(model string) error      { return saveValue("image_model", model) }
func SaveS2TBaseURL(url string) error        { return saveValue("s2t_base_url", url) }
func SaveT2SBaseURL(url string) error        { return saveValue("t2s_base_url", url) }
func SaveImageBaseURL(url string) error      { return saveValue("image_base_url", url) }
func SaveRealtimeURL(url string) error       { return saveValue("realtime_url", url) }
func SaveRealtimeModel(model string) error   { return saveValue("realtime_model", model) }
func SaveOpenClawURL(url string) error       { return saveValue("openclaw_url", url) }
func SaveOpenClawToken(token string) error   { return saveValue("openclaw_token", token) }
func SaveOpenClawAgent(agent string) error   { return saveValue("openclaw_agent", agent) }
func SaveOpenClawPassword(pass string) error { return saveValue("openclaw_password", pass) }
