package telegram

import (
	"errors"
	"os"
	"strconv"
)

type Config struct {
	BotToken      string
	DefaultChatID int64
	Enabled       bool
}

func LoadConfig() (*Config, error) {
	token := os.Getenv("TELEGRAM_BOT_TOKEN")
	chatIDStr := os.Getenv("TELEGRAM_CHAT_ID")

	cfg := &Config{
		BotToken: token,
		Enabled:  token != "",
	}

	if chatIDStr != "" {
		chatID, err := strconv.ParseInt(chatIDStr, 10, 64)
		if err != nil {
			return nil, errors.New("invalid TELEGRAM_CHAT_ID format")
		}
		cfg.DefaultChatID = chatID
	}

	return cfg, nil
}

func (c *Config) Validate() error {
	if c.Enabled && c.BotToken == "" {
		return errors.New("TELEGRAM_BOT_TOKEN is required when telegram is enabled")
	}
	return nil
}
