package main

import (
	"os"
	"qq2rss/config"
	"qq2rss/server"
)

func main() {
	configPath := "config.json"
	if len(os.Args) > 1 {
		configPath = os.Args[1]
	}

	cfg, err := config.Load(configPath)
	if err != nil {
		panic(err)
	}

	s, err := server.NewServer(server.Config{
		Title:       cfg.Feed.Title,
		Link:        cfg.Feed.Link,
		Description: cfg.Feed.Description,
		AuthorName:  cfg.Feed.AuthorName,
		AuthorEmail: cfg.Feed.AuthorEmail,
		StoragePath: cfg.Feed.StoragePath,
		MaxItems:    cfg.Feed.MaxItems,
		GroupID:     cfg.Feed.GroupID,
		OneBotToken: cfg.Feed.OneBotToken,
	})
	if err != nil {
		panic(err)
	}

	if err := s.Start(cfg.ListenAddr); err != nil {
		panic(err)
	}
}
