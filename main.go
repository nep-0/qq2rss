package main

import (
	"fmt"
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
	})
	if err != nil {
		panic(err)
	}

	errCh := make(chan error, 2)
	go func() {
		errCh <- s.Start(cfg.ListenAddr)
	}()
	go func() {
		errCh <- s.StartOneBot(cfg.OneBotListenAddr)
	}()

	if err := <-errCh; err != nil {
		panic(fmt.Errorf("server stopped: %w", err))
	}
}
