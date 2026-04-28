package config

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/viper"
)

const (
	DefaultModel      = "gpt-4o-mini"
	DefaultTemp       = 0.7
	DefaultMaxTokens  = 2048
	DefaultCostLimit  = 0.0 // 0 = unlimited
	DefaultWarnPct    = 80.0
	DefaultImageModel = "gpt-image-1"
	DefaultImageSize  = "1024x1024"
	DefaultBaseURL    = "https://api.openai.com/v1"
	DefaultS2TModel   = "gpt-4o-mini-transcribe"
	DefaultT2SModel   = "gpt-4o-mini-tts"
	DefaultT2SVoice   = "marin"
)

func Init() {
	viper.SetConfigName("config")
	viper.SetConfigType("yaml")

	configDir := ConfigDir()
	viper.AddConfigPath(configDir)

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

func MCPServers() map[string]interface{} {
	return viper.GetStringMap("mcp_servers")
}

func SaveAPIBaseURL(url string) error {
	dir := ConfigDir()
	if err := os.MkdirAll(dir, 0700); err != nil {
		return fmt.Errorf("create config dir: %w", err)
	}

	viper.Set("api_base_url", url)

	cfgFile := ConfigFile()
	if err := viper.WriteConfigAs(cfgFile); err != nil {
		return fmt.Errorf("write config: %w", err)
	}
	return os.Chmod(cfgFile, 0600)
}

func SaveAPIKey(key string) error {
	dir := ConfigDir()
	if err := os.MkdirAll(dir, 0700); err != nil {
		return fmt.Errorf("create config dir: %w", err)
	}

	viper.Set("api_key", key)

	cfgFile := ConfigFile()
	if err := viper.WriteConfigAs(cfgFile); err != nil {
		return fmt.Errorf("write config: %w", err)
	}
	return os.Chmod(cfgFile, 0600)
}
