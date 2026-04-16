package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/gorilla/feeds"
)

// Config controls feed metadata and persistence behavior.
type Config struct {
	Title       string
	Link        string
	Description string
	AuthorName  string
	AuthorEmail string
	StoragePath string
	MaxItems    int
}

// Item is a persisted feed item.
type Item struct {
	ID          string    `json:"id"`
	Title       string    `json:"title"`
	Link        string    `json:"link"`
	Description string    `json:"description"`
	Content     string    `json:"content"`
	AuthorName  string    `json:"author_name"`
	AuthorEmail string    `json:"author_email"`
	Created     time.Time `json:"created"`
}

type diskState struct {
	Items []Item `json:"items"`
}

// Server persists feed items on disk and serves them as RSS.
type Server struct {
	mu         sync.RWMutex
	cfg        Config
	items      []Item
	httpServer *http.Server
}

// NewServer loads persisted state and returns a ready-to-use server.
func NewServer(cfg Config) (*Server, error) {
	normalized, err := normalizeConfig(cfg)
	if err != nil {
		return nil, err
	}

	loadedItems, err := loadItems(normalized.StoragePath)
	if err != nil {
		return nil, err
	}

	s := &Server{
		cfg:   normalized,
		items: loadedItems,
	}

	if len(s.items) > s.cfg.MaxItems {
		s.items = s.items[:s.cfg.MaxItems]
		if err := saveItems(s.cfg.StoragePath, s.items); err != nil {
			return nil, err
		}
	}

	return s, nil
}

// AddItem prepends a new item, enforces retention, and persists to disk.
func (s *Server) AddItem(item Item) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if item.Title == "" {
		return errors.New("item title is required")
	}
	if item.Link == "" {
		return errors.New("item link is required")
	}
	if item.Created.IsZero() {
		item.Created = time.Now().UTC()
	}
	if item.ID == "" {
		item.ID = fmt.Sprintf("%s-%d", item.Link, item.Created.UnixNano())
	}

	s.items = append([]Item{item}, s.items...)
	if len(s.items) > s.cfg.MaxItems {
		s.items = s.items[:s.cfg.MaxItems]
	}

	return saveItems(s.cfg.StoragePath, s.items)
}

// Items returns a copy of the currently stored items.
func (s *Server) Items() []Item {
	s.mu.RLock()
	defer s.mu.RUnlock()

	out := make([]Item, len(s.items))
	copy(out, s.items)
	return out
}

// RSS returns the feed serialized as RSS 2.0 XML.
func (s *Server) RSS() (string, error) {
	s.mu.RLock()
	cfg := s.cfg
	itemsCopy := make([]Item, len(s.items))
	copy(itemsCopy, s.items)
	s.mu.RUnlock()

	feed := &feeds.Feed{
		Title:       cfg.Title,
		Link:        &feeds.Link{Href: cfg.Link},
		Description: cfg.Description,
		Author: &feeds.Author{
			Name:  cfg.AuthorName,
			Email: cfg.AuthorEmail,
		},
		Created: time.Now().UTC(),
	}

	feedItems := make([]*feeds.Item, 0, len(itemsCopy))
	for _, item := range itemsCopy {
		current := item
		feedItems = append(feedItems, &feeds.Item{
			Id:          current.ID,
			Title:       current.Title,
			Link:        &feeds.Link{Href: current.Link},
			Description: current.Description,
			Content:     current.Content,
			Author: &feeds.Author{
				Name:  current.AuthorName,
				Email: current.AuthorEmail,
			},
			Created: current.Created,
		})
	}

	if len(feedItems) > 0 {
		feed.Created = feedItems[0].Created
	}
	feed.Items = feedItems

	return feed.ToRss()
}

// Handler returns an HTTP handler with endpoints for RSS and adding items.
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/rss", s.handleRSS)
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})
	return mux
}

// Start runs the HTTP server and blocks until it stops.
func (s *Server) Start(addr string) error {
	if addr == "" {
		addr = ":8080"
	}

	s.mu.Lock()
	if s.httpServer != nil {
		s.mu.Unlock()
		return errors.New("server already started")
	}
	s.httpServer = &http.Server{
		Addr:    addr,
		Handler: s.Handler(),
	}
	srv := s.httpServer
	s.mu.Unlock()

	err := srv.ListenAndServe()
	if errors.Is(err, http.ErrServerClosed) {
		return nil
	}
	return err
}

// Shutdown gracefully stops a running server.
func (s *Server) Shutdown(ctx context.Context) error {
	s.mu.Lock()
	srv := s.httpServer
	s.httpServer = nil
	s.mu.Unlock()

	if srv == nil {
		return nil
	}
	return srv.Shutdown(ctx)
}

func (s *Server) handleRSS(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	rss, err := s.RSS()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/rss+xml; charset=utf-8")
	_, _ = w.Write([]byte(rss))
}

func normalizeConfig(cfg Config) (Config, error) {
	if cfg.MaxItems <= 0 {
		return Config{}, errors.New("max items must be greater than zero")
	}
	if cfg.StoragePath == "" {
		cfg.StoragePath = "rss_items.json"
	}
	if cfg.Title == "" {
		cfg.Title = "qq2rss feed"
	}
	if cfg.Link == "" {
		cfg.Link = "http://localhost"
	}
	if cfg.Description == "" {
		cfg.Description = "persistent rss feed"
	}
	return cfg, nil
}

func loadItems(path string) ([]Item, error) {
	content, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return []Item{}, nil
	}
	if err != nil {
		return nil, err
	}
	if len(content) == 0 {
		return []Item{}, nil
	}

	var state diskState
	if err := json.Unmarshal(content, &state); err != nil {
		return nil, err
	}
	if state.Items == nil {
		return []Item{}, nil
	}
	return state.Items, nil
}

func saveItems(path string, items []Item) error {
	state := diskState{Items: items}
	content, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return err
	}

	dir := filepath.Dir(path)
	if dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return err
		}
	}

	return os.WriteFile(path, content, 0o644)
}
