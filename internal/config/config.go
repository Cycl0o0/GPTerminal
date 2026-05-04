package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/viper"
)

const (
	DefaultModel               = "gpt-4o-mini"
	DefaultTemp                = 0.7
	DefaultMaxTokens           = 2048
	DefaultCostLimit           = 0.0 // 0 = unlimited
	DefaultWarnPct             = 80.0
	DefaultImageModel          = "gpt-image-1"
	DefaultImageSize           = "1024x1024"
	DefaultBaseURL             = "https://api.openai.com/v1"
	DefaultS2TModel            = "gpt-4o-mini-transcribe"
	DefaultT2SModel            = "gpt-4o-mini-tts"
	DefaultT2SVoice            = "marin"
	DefaultRealtimeSessionModel = "gpt-realtime"
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

	viper.SetEnvPrefix("OPENAI")
	viper.BindEnv("api_key")
	viper.BindEnv("model", "OPENAI_MODEL")
	viper.BindEnv("api_base_url", "OPENAI_API_BASE_URL")

	viper.BindEnv("anthropic_api_key", "ANTHROPIC_API_KEY")
	viper.BindEnv("gemini_api_key", "GEMINI_API_KEY")
	viper.BindEnv("provider", "GPTERMINAL_PROVIDER")

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

func MCPServers() map[string]interface{} {
	return viper.GetStringMap("mcp_servers")
}

// saveValue is a shared helper used by all Save* functions.
func saveValue(key, value string) error {
	dir := ConfigDir()
	if err := os.MkdirAll(dir, 0700); err != nil {
		return fmt.Errorf("create config dir: %w", err)
	}
	viper.Set(key, value)
	cfgFile := ConfigFile()
	if err := viper.WriteConfigAs(cfgFile); err != nil {
		return fmt.Errorf("write config: %w", err)
	}
	return os.Chmod(cfgFile, 0600)
}

func SaveProvider(provider string) error     { return saveValue("provider", provider) }
func SaveAnthropicAPIKey(key string) error   { return saveValue("anthropic_api_key", key) }
func SaveGeminiAPIKey(key string) error      { return saveValue("gemini_api_key", key) }
func SaveAPIBaseURL(url string) error        { return saveValue("api_base_url", url) }
func SaveModel(model string) error     { return saveValue("model", model) }
func SaveAPIKey(key string) error      { return saveValue("api_key", key) }
func SaveS2TModel(model string) error  { return saveValue("s2t_model", model) }
func SaveT2SModel(model string) error  { return saveValue("t2s_model", model) }
func SaveT2SVoice(voice string) error  { return saveValue("t2s_voice", voice) }
func SaveImageModel(model string) error { return saveValue("image_model", model) }
func SaveS2TBaseURL(url string) error  { return saveValue("s2t_base_url", url) }
func SaveT2SBaseURL(url string) error  { return saveValue("t2s_base_url", url) }
func SaveImageBaseURL(url string) error { return saveValue("image_base_url", url) }
func SaveRealtimeURL(url string) error { return saveValue("realtime_url", url) }
func SaveRealtimeModel(model string) error { return saveValue("realtime_model", model) }
