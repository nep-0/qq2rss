package server

import (
	"crypto/subtle"
	"encoding/json"
	"html"
	"io"
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
var titleTagPattern = regexp.MustCompile(`(?is)<title[^>]*>(.*?)</title>`)
var metaTagPattern = regexp.MustCompile(`(?is)<meta\s+[^>]*>`)
var attrPattern = regexp.MustCompile(`(?i)([a-zA-Z_:][-a-zA-Z0-9_:.]*)\s*=\s*"([^"]*)"|([a-zA-Z_:][-a-zA-Z0-9_:.]*)\s*=\s*'([^']*)'`)

type linkMetadata struct {
	Title       string
	Description string
}

var fetchLinkMetadata = defaultFetchLinkMetadata

func defaultFetchLinkMetadata(link string) (linkMetadata, error) {
	req, err := http.NewRequest(http.MethodGet, link, nil)
	if err != nil {
		return linkMetadata{}, err
	}
	req.Header.Set("User-Agent", "qq2rss/1.0")

	client := &http.Client{Timeout: 8 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return linkMetadata{}, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 2<<20))
	if err != nil {
		return linkMetadata{}, err
	}

	return parseHTMLMetadata(string(body)), nil
}

func (s *Server) handleOneBot(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	if !isAuthorizedOneBotRequest(r, s.cfg.OneBotToken) {
		w.WriteHeader(http.StatusUnauthorized)
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

	meta, _ := fetchLinkMetadata(link)
	title := chooseNonEmpty(meta.Title, makeTitle(text, link))
	description := chooseNonEmpty(meta.Description, text)
	content := chooseNonEmpty(meta.Description, text)

	id, err := uuid.NewV7()
	if err != nil {
		http.Error(w, "failed to generate item ID", http.StatusInternalServerError)
		return
	}

	if err := s.AddItem(Item{
		ID:          id.String(),
		Title:       title,
		Link:        link,
		Description: description,
		Content:     content,
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

func parseHTMLMetadata(body string) linkMetadata {
	meta := linkMetadata{}

	for _, tag := range metaTagPattern.FindAllString(body, -1) {
		attrs := parseAttributes(tag)
		content := strings.TrimSpace(html.UnescapeString(attrs["content"]))
		if content == "" {
			continue
		}

		name := strings.ToLower(strings.TrimSpace(attrs["name"]))
		property := strings.ToLower(strings.TrimSpace(attrs["property"]))

		switch {
		case meta.Title == "" && (property == "og:title" || name == "twitter:title"):
			meta.Title = content
		case meta.Description == "" && (name == "description" || property == "og:description" || name == "twitter:description"):
			meta.Description = content
		}
	}

	if meta.Title == "" {
		match := titleTagPattern.FindStringSubmatch(body)
		if len(match) > 1 {
			meta.Title = strings.TrimSpace(html.UnescapeString(match[1]))
		}
	}

	return meta
}

func parseAttributes(tag string) map[string]string {
	attrs := make(map[string]string)
	matches := attrPattern.FindAllStringSubmatch(tag, -1)
	for _, m := range matches {
		if len(m) < 5 {
			continue
		}

		key := strings.TrimSpace(m[1])
		value := strings.TrimSpace(m[2])
		if key == "" {
			key = strings.TrimSpace(m[3])
			value = strings.TrimSpace(m[4])
		}
		if key == "" {
			continue
		}

		attrs[strings.ToLower(key)] = value
	}
	return attrs
}

func chooseNonEmpty(primary string, fallback string) string {
	if strings.TrimSpace(primary) != "" {
		return strings.TrimSpace(primary)
	}
	return strings.TrimSpace(fallback)
}

func isAuthorizedOneBotRequest(r *http.Request, configuredToken string) bool {
	reqToken := extractRequestToken(r)
	if reqToken == "" || configuredToken == "" {
		return false
	}

	reqBytes := []byte(reqToken)
	cfgBytes := []byte(configuredToken)
	if len(reqBytes) != len(cfgBytes) {
		return false
	}

	return subtle.ConstantTimeCompare(reqBytes, cfgBytes) == 1
}

func extractRequestToken(r *http.Request) string {
	auth := strings.TrimSpace(r.Header.Get("Authorization"))
	if strings.HasPrefix(strings.ToLower(auth), "bearer ") {
		token := strings.TrimSpace(auth[len("Bearer "):])
		if token != "" {
			return token
		}
	}

	return strings.TrimSpace(r.Header.Get("X-OneBot-Token"))
}
