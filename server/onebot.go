package server

import (
	"encoding/json"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"
)

type oneBotSender struct {
	Nickname string `json:"nickname"`
}

type oneBotEvent struct {
	PostType    string          `json:"post_type"`
	MessageType string          `json:"message_type"`
	GroupID     int64           `json:"group_id"`
	RawMessage  string          `json:"raw_message"`
	Message     json.RawMessage `json:"message"`
	Time        int64           `json:"time"`
	Sender      oneBotSender    `json:"sender"`
}

type oneBotSegment struct {
	Type string                 `json:"type"`
	Data map[string]interface{} `json:"data"`
}

var urlPattern = regexp.MustCompile(`https?://[^\s<>"']+`)

func (s *Server) handleOneBot(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	defer r.Body.Close()
	var event oneBotEvent
	if err := json.NewDecoder(r.Body).Decode(&event); err != nil {
		http.Error(w, "invalid onebot payload", http.StatusBadRequest)
		return
	}

	if event.PostType != "message" || event.MessageType != "group" {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	if event.GroupID != s.cfg.GroupID {
		w.WriteHeader(http.StatusNoContent)
		return
	}

	text := extractOneBotText(event)
	link, ok := extractFirstLink(text)
	if !ok {
		w.WriteHeader(http.StatusNoContent)
		return
	}

	created := time.Now().UTC()
	if event.Time > 0 {
		created = time.Unix(event.Time, 0).UTC()
	}

	id, err := uuid.NewV7()
	if err != nil {
		http.Error(w, "failed to generate item ID", http.StatusInternalServerError)
		return
	}

	if err := s.AddItem(Item{
		ID:          id.String(),
		Title:       makeTitle(text, link),
		Link:        link,
		Description: text,
		Content:     text,
		AuthorName:  event.Sender.Nickname,
		Created:     created,
	}); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	w.WriteHeader(http.StatusAccepted)
}

func extractOneBotText(event oneBotEvent) string {
	if strings.TrimSpace(event.RawMessage) != "" {
		return strings.TrimSpace(event.RawMessage)
	}

	if len(event.Message) == 0 {
		return ""
	}

	var plain string
	if err := json.Unmarshal(event.Message, &plain); err == nil {
		return strings.TrimSpace(plain)
	}

	var segments []oneBotSegment
	if err := json.Unmarshal(event.Message, &segments); err != nil {
		return ""
	}

	parts := make([]string, 0, len(segments))
	for _, seg := range segments {
		switch seg.Type {
		case "text":
			if txt, ok := seg.Data["text"].(string); ok && strings.TrimSpace(txt) != "" {
				parts = append(parts, txt)
			}
		case "link":
			if u, ok := seg.Data["url"].(string); ok && strings.TrimSpace(u) != "" {
				parts = append(parts, u)
			}
		}
	}

	return strings.TrimSpace(strings.Join(parts, " "))
}

func extractFirstLink(text string) (string, bool) {
	matches := urlPattern.FindStringSubmatch(text)
	if len(matches) == 0 {
		return "", false
	}
	parsed, err := url.Parse(matches[0])
	if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Host == "" {
		return "", false
	}
	return parsed.String(), true
}

func makeTitle(text string, fallback string) string {
	trimmed := strings.TrimSpace(text)
	if trimmed == "" {
		return fallback
	}
	if len(trimmed) <= 120 {
		return trimmed
	}
	return trimmed[:120]
}
