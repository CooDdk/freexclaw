package config

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/viper"
)

type Config struct {
	APIKey       string `mapstructure:"api_key"`
	BaseURL      string `mapstructure:"base_url"`
	Model        string `mapstructure:"model"`
	SystemPrompt string `mapstructure:"system_prompt"`
}

var defaultConfig = Config{
	BaseURL:      "https://api.openai.com/v1",
	Model:        "gpt-4o",
	SystemPrompt: "你是一个乐于助人的 AI 助手。请用简洁、准确的语言回答用户的问题。",
}

func Load() (*Config, error) {
	configDir, err := os.UserConfigDir()
	if err != nil {
		return nil, fmt.Errorf("获取配置目录失败: %w", err)
	}

	appConfigDir := filepath.Join(configDir, "FREEXCLAW")
	if err := os.MkdirAll(appConfigDir, 0755); err != nil {
		return nil, fmt.Errorf("创建配置目录失败: %w", err)
	}

	viper.SetConfigName("config")
	viper.SetConfigType("yaml")
	viper.AddConfigPath(appConfigDir)

	viper.SetDefault("api_key", defaultConfig.APIKey)
	viper.SetDefault("base_url", defaultConfig.BaseURL)
	viper.SetDefault("model", defaultConfig.Model)
	viper.SetDefault("system_prompt", defaultConfig.SystemPrompt)

	if err := viper.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); ok {
			configFile := filepath.Join(appConfigDir, "config.yaml")
			if err := viper.WriteConfigAs(configFile); err != nil {
				return nil, fmt.Errorf("创建默认配置文件失败: %w", err)
			}
		} else {
			return nil, fmt.Errorf("读取配置文件失败: %w", err)
		}
	}

	var cfg Config
	if err := viper.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("解析配置失败: %w", err)
	}

	return &cfg, nil
}

func Save(cfg *Config) error {
	viper.Set("api_key", cfg.APIKey)
	viper.Set("base_url", cfg.BaseURL)
	viper.Set("model", cfg.Model)
	viper.Set("system_prompt", cfg.SystemPrompt)

	return viper.WriteConfig()
}

func GetConfigPath() (string, error) {
	configDir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(configDir, "FREEXCLAW", "config.yaml"), nil
}
