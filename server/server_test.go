package server

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestAddItemPersistsAndCaps(t *testing.T) {
	storagePath := filepath.Join(t.TempDir(), "feed-state.json")
	cfg := Config{
		Title:       "Test feed",
		Link:        "http://localhost",
		Description: "test",
		StoragePath: storagePath,
		MaxItems:    2,
	}

	s, err := NewServer(cfg)
	if err != nil {
		t.Fatalf("NewServer returned error: %v", err)
	}

	if err := s.AddItem(Item{Title: "one", Link: "http://localhost/1", Created: time.Unix(1, 0).UTC()}); err != nil {
		t.Fatalf("AddItem(one) returned error: %v", err)
	}
	if err := s.AddItem(Item{Title: "two", Link: "http://localhost/2", Created: time.Unix(2, 0).UTC()}); err != nil {
		t.Fatalf("AddItem(two) returned error: %v", err)
	}
	if err := s.AddItem(Item{Title: "three", Link: "http://localhost/3", Created: time.Unix(3, 0).UTC()}); err != nil {
		t.Fatalf("AddItem(three) returned error: %v", err)
	}

	items := s.Items()
	if got, want := len(items), 2; got != want {
		t.Fatalf("unexpected item count in memory: got %d want %d", got, want)
	}
	if got, want := items[0].Title, "three"; got != want {
		t.Fatalf("unexpected newest item title: got %q want %q", got, want)
	}
	if got, want := items[1].Title, "two"; got != want {
		t.Fatalf("unexpected second item title: got %q want %q", got, want)
	}

	persisted, err := loadItems(storagePath)
	if err != nil {
		t.Fatalf("loadItems returned error: %v", err)
	}
	if got, want := len(persisted), 2; got != want {
		t.Fatalf("unexpected item count on disk: got %d want %d", got, want)
	}

	reloaded, err := NewServer(cfg)
	if err != nil {
		t.Fatalf("NewServer(reload) returned error: %v", err)
	}
	reloadedItems := reloaded.Items()
	if got, want := len(reloadedItems), 2; got != want {
		t.Fatalf("unexpected item count after reload: got %d want %d", got, want)
	}
	if got, want := reloadedItems[0].Title, "three"; got != want {
		t.Fatalf("unexpected newest item after reload: got %q want %q", got, want)
	}
}

func TestRSSOutput(t *testing.T) {
	cfg := Config{
		Title:       "Test feed",
		Link:        "http://localhost",
		Description: "test",
		StoragePath: filepath.Join(t.TempDir(), "feed-state.json"),
		MaxItems:    5,
	}

	s, err := NewServer(cfg)
	if err != nil {
		t.Fatalf("NewServer returned error: %v", err)
	}

	if err := s.AddItem(Item{Title: "hello", Link: "http://localhost/hello", Description: "world", Content: "full item content"}); err != nil {
		t.Fatalf("AddItem returned error: %v", err)
	}

	rss, err := s.RSS()
	if err != nil {
		t.Fatalf("RSS returned error: %v", err)
	}
	if !strings.Contains(rss, "<rss") {
		t.Fatalf("rss output missing rss root element: %s", rss)
	}
	if !strings.Contains(rss, "<title>hello</title>") {
		t.Fatalf("rss output missing item title: %s", rss)
	}
	if !strings.Contains(rss, "<content:encoded><![CDATA[full item content]]></content:encoded>") {
		t.Fatalf("rss output missing item content: %s", rss)
	}
}

func TestLoadItemsWithInvalidJSON(t *testing.T) {
	path := filepath.Join(t.TempDir(), "bad.json")
	if err := os.WriteFile(path, []byte("not json"), 0o644); err != nil {
		t.Fatalf("WriteFile returned error: %v", err)
	}

	if _, err := loadItems(path); err == nil {
		t.Fatal("expected loadItems to fail for invalid JSON")
	}
}
