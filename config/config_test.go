package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoad(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")
	json := `{
		"listen_addr": ":9000",
		"feed": {
			"title": "feed",
			"link": "https://example.com",
			"description": "desc",
			"author_name": "name",
			"author_email": "a@example.com",
			"storage_path": "store.json",
			"max_items": 20,
			"group_id": 123,
			"onebot_token": "test-token"
		}
	}`
	if err := os.WriteFile(path, []byte(json), 0o644); err != nil {
		t.Fatalf("WriteFile returned error: %v", err)
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	if got, want := cfg.ListenAddr, ":9000"; got != want {
		t.Fatalf("unexpected listen addr: got %q want %q", got, want)
	}
	if got, want := cfg.Feed.GroupID, int64(123); got != want {
		t.Fatalf("unexpected group id: got %d want %d", got, want)
	}
}

func TestLoadValidationFailure(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")
	json := `{
		"listen_addr": ":9000",
		"feed": {
			"storage_path": "store.json",
			"max_items": 0,
			"group_id": 0,
			"onebot_token": ""
		}
	}`
	if err := os.WriteFile(path, []byte(json), 0o644); err != nil {
		t.Fatalf("WriteFile returned error: %v", err)
	}

	if _, err := Load(path); err == nil {
		t.Fatal("expected Load to fail validation")
	}
}
