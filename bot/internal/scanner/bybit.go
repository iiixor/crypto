package scanner

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"crypto-bot/internal/model"
)

const (
	bybitSource     = "bybit"
	bybitListingURL = "https://api.bybit.com/v5/announcements/index?locale=en-US&limit=20&type=new_crypto"
)

type bybitResponse struct {
	Result struct {
		List []bybitAnnouncement `json:"list"`
	} `json:"result"`
}

type bybitAnnouncement struct {
	Title         string `json:"title"`
	Description   string `json:"description"`
	DateTimestamp int64  `json:"dateTimestamp"` // ms UTC — дата публикации
	URL           string `json:"url"`
}

type BybitScanner struct {
	client *http.Client
}

func NewBybitScanner() *BybitScanner {
	return &BybitScanner{
		client: &http.Client{Timeout: 15 * time.Second},
	}
}

func (s *BybitScanner) Scan(ctx context.Context) ([]model.Event, error) {
	from := time.Now().UTC().Add(-14 * 24 * time.Hour)
	to := time.Now().UTC().Add(7 * 24 * time.Hour)

	items, err := s.fetchAnnouncements(ctx)
	if err != nil {
		log.Printf("[bybit] warning: %v", err)
		return []model.Event{}, nil
	}

	var events []model.Event
	for _, a := range items {
		ev, ok := s.parseAnnouncement(a, from, to)
		if !ok {
			continue
		}
		events = append(events, ev)
	}
	return deduplicateEvents(events), nil
}

func (s *BybitScanner) fetchAnnouncements(ctx context.Context) ([]bybitAnnouncement, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, bybitListingURL, nil)
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Accept", "application/json")

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("do request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("status %d from bybit", resp.StatusCode)
	}

	var parsed bybitResponse
	if err := json.NewDecoder(resp.Body).Decode(&parsed); err != nil {
		return nil, fmt.Errorf("decode: %w", err)
	}
	return parsed.Result.List, nil
}

func (s *BybitScanner) parseAnnouncement(a bybitAnnouncement, from, to time.Time) (model.Event, bool) {
	pubDate := time.Unix(a.DateTimestamp/1000, 0).UTC()

	eventDate, _ := extractEventDateFromTitle(a.Title, pubDate)

	if eventDate.Before(from) || eventDate.After(to) {
		return model.Event{}, false
	}

	eventType, ok := classifyBybitTitle(a.Title)
	if !ok {
		return model.Event{}, false
	}

	token := extractTokenFromParentheses(a.Title)
	if token == "" {
		token = extractTokenFromTitle(a.Title)
	}
	if token == "" {
		token = "UNKNOWN"
	}

	return model.Event{
		ID:      makeEventID(bybitSource, token, eventDate),
		Type:    eventType,
		Source:  bybitSource,
		Token:   strings.ToUpper(token),
		Title:   a.Title,
		Date:    eventDate,
		URL:     a.URL,
		Details: truncate(a.Description, 200),
	}, true
}

func classifyBybitTitle(title string) (model.EventType, bool) {
	upper := strings.ToUpper(title)
	switch {
	case strings.Contains(upper, "LAUNCHPOOL"):
		return model.EventLaunchpool, true
	case strings.Contains(upper, "LIST") || strings.Contains(upper, "PERPETUAL") ||
		strings.Contains(upper, "FUTURES") || strings.Contains(upper, "CONVERT"):
		return model.EventListing, true
	default:
		return "", false
	}
}
