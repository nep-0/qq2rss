package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
)

// Config contains application runtime configuration loaded from JSON.
type Config struct {
	ListenAddr  string     `json:"listen_addr"`
	Feed        FeedConfig `json:"feed"`
	OneBotPath  string     `json:"onebot_path"`
	HealthzPath string     `json:"healthz_path"`
	RSSPath     string     `json:"rss_path"`
}

// FeedConfig maps to rss server behavior.
type FeedConfig struct {
	Title       string `json:"title"`
	Link        string `json:"link"`
	Description string `json:"description"`
	AuthorName  string `json:"author_name"`
	AuthorEmail string `json:"author_email"`
	StoragePath string `json:"storage_path"`
	MaxItems    int    `json:"max_items"`
	GroupID     int64  `json:"group_id"`
	OneBotToken string `json:"onebot_token"`
}

// Load reads and validates configuration from a JSON file.
func Load(path string) (Config, error) {
	if strings.TrimSpace(path) == "" {
		return Config{}, errors.New("config path is required")
	}

	content, err := os.ReadFile(path)
	if err != nil {
		return Config{}, fmt.Errorf("read config: %w", err)
	}

	cfg := Config{
		ListenAddr:  ":8080",
		RSSPath:     "/rss",
		HealthzPath: "/healthz",
		OneBotPath:  "/onebot",
	}
	if err := json.Unmarshal(content, &cfg); err != nil {
		return Config{}, fmt.Errorf("parse config JSON: %w", err)
	}

	if err := cfg.Validate(); err != nil {
		return Config{}, err
	}
	return cfg, nil
}

// Validate checks required config fields.
func (c Config) Validate() error {
	if strings.TrimSpace(c.ListenAddr) == "" {
		return errors.New("listen_addr is required")
	}
	if strings.TrimSpace(c.Feed.StoragePath) == "" {
		return errors.New("feed.storage_path is required")
	}
	if c.Feed.MaxItems <= 0 {
		return errors.New("feed.max_items must be greater than zero")
	}
	if c.Feed.GroupID <= 0 {
		return errors.New("feed.group_id must be greater than zero")
	}
	if strings.TrimSpace(c.Feed.OneBotToken) == "" {
		return errors.New("feed.onebot_token is required")
	}
	return nil
}
