package config

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/viper"
)

const (
	DefaultModel     = "gpt-4o-mini"
	DefaultTemp      = 0.7
	DefaultMaxTokens = 2048
)

func Init() {
	viper.SetConfigName("config")
	viper.SetConfigType("yaml")

	configDir := ConfigDir()
	viper.AddConfigPath(configDir)

	viper.SetDefault("model", DefaultModel)
	viper.SetDefault("temperature", DefaultTemp)
	viper.SetDefault("max_tokens", DefaultMaxTokens)

	viper.SetEnvPrefix("OPENAI")
	viper.BindEnv("api_key")
	viper.BindEnv("model", "OPENAI_MODEL")

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
