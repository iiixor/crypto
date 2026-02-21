package config

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Telegram TelegramConfig `yaml:"telegram"`
	Schedule ScheduleConfig `yaml:"schedule"`
	Scanner  ScannerConfig  `yaml:"scanner"`
	Sources  SourcesConfig  `yaml:"sources"`
}

type TelegramConfig struct {
	BotToken string `yaml:"bot_token"`
	ChatID   string `yaml:"chat_id"`
}

type ScheduleConfig struct {
	DigestWeekday string `yaml:"digest_weekday"`
	DigestTimeUTC string `yaml:"digest_time_utc"`
}

type ScannerConfig struct {
	RefreshIntervalMinutes int `yaml:"refresh_interval_minutes"`
}

type SourcesConfig struct {
	Bybit        bool `yaml:"bybit"`
	Binance      bool `yaml:"binance"`
	OKX          bool `yaml:"okx"`
	TokenUnlocks bool `yaml:"tokenunlocks"`
	Airdrops     bool `yaml:"airdrops"`
}

func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config: %w", err)
	}
	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}
	if cfg.Scanner.RefreshIntervalMinutes == 0 {
		cfg.Scanner.RefreshIntervalMinutes = 60
	}
	return &cfg, nil
}
