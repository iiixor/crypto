package scanner

import (
	"context"
	"encoding/xml"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"

	"crypto-bot/internal/model"
)

const (
	airdropsSource = "airdrops"
	airdropsRSSURL = "https://airdrops.io/feed/"
)

// rssFeed represents the top-level RSS document.
type rssFeed struct {
	XMLName xml.Name   `xml:"rss"`
	Channel rssChannel `xml:"channel"`
}

// rssChannel holds the list of items within an RSS feed.
type rssChannel struct {
	Items []rssItem `xml:"item"`
}

// rssItem represents a single entry in the RSS feed.
type rssItem struct {
	Title       string `xml:"title"`
	Link        string `xml:"link"`
	PubDate     string `xml:"pubDate"`
	Description string `xml:"description"`
}

// AirdropsScanner fetches airdrop events from the airdrops.io RSS feed.
type AirdropsScanner struct {
	client *http.Client
}

// NewAirdropsScanner constructs an AirdropsScanner with a sensible HTTP client.
func NewAirdropsScanner() *AirdropsScanner {
	return &AirdropsScanner{
		client: &http.Client{Timeout: 15 * time.Second},
	}
}

// Scan fetches the airdrops.io RSS feed and returns events within the next 7 days.
// If the feed is unavailable the scanner logs a warning and returns an empty
// (non-nil) slice — callers should treat this as a graceful degradation.
func (s *AirdropsScanner) Scan(ctx context.Context) ([]model.Event, error) {
	// Окно: статьи опубликованы за последние 14 дней (дата публикации = дата события)
	from := time.Now().UTC().Add(-14 * 24 * time.Hour)
	horizon := time.Now().UTC().Add(7 * 24 * time.Hour)

	items, err := s.fetchFeed(ctx)
	if err != nil {
		log.Printf("[airdrops] warning: failed to fetch RSS feed: %v", err)
		return []model.Event{}, nil
	}

	var events []model.Event
	for _, item := range items {
		ev, ok := parseRSSItem(item, from, horizon)
		if !ok {
			continue
		}
		events = append(events, ev)
	}

	return events, nil
}

// fetchFeed performs the HTTP GET and parses the RSS XML from airdrops.io.
func (s *AirdropsScanner) fetchFeed(ctx context.Context) ([]rssItem, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, airdropsRSSURL, nil)
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Accept", "application/rss+xml, application/xml, text/xml")
	req.Header.Set("User-Agent", binanceUserAgent)

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("do request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status %d from airdrops.io", resp.StatusCode)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, 2<<20)) // 2 MB cap
	if err != nil {
		return nil, fmt.Errorf("read body: %w", err)
	}

	var feed rssFeed
	if err := xml.Unmarshal(body, &feed); err != nil {
		return nil, fmt.Errorf("parse XML: %w", err)
	}

	return feed.Channel.Items, nil
}

// parseRSSItem converts a raw RSS item into a model.Event.
// Returns (event, false) when the item should be skipped.
func parseRSSItem(item rssItem, now, horizon time.Time) (model.Event, bool) {
	if item.PubDate == "" {
		return model.Event{}, false
	}

	eventDate, err := parseRSSDate(item.PubDate)
	if err != nil {
		log.Printf("[airdrops] warning: cannot parse pubDate %q: %v", item.PubDate, err)
		return model.Event{}, false
	}
	eventDate = eventDate.UTC()

	if eventDate.Before(now) || eventDate.After(horizon) {
		return model.Event{}, false
	}

	token := extractTokenFromTitle(item.Title)
	if token == "" {
		token = "UNKNOWN"
	}

	details := cleanDescription(item.Description)

	return model.Event{
		ID:      makeEventID(airdropsSource, token, eventDate),
		Type:    model.EventAirdrop,
		Source:  airdropsSource,
		Token:   strings.ToUpper(token),
		Title:   strings.TrimSpace(item.Title),
		Date:    eventDate,
		URL:     strings.TrimSpace(item.Link),
		Details: details,
	}, true
}

// parseRSSDate tries RFC1123Z first, then several fallback layouts that
// real-world RSS feeds commonly use.
func parseRSSDate(s string) (time.Time, error) {
	s = strings.TrimSpace(s)
	layouts := []string{
		time.RFC1123Z,          // "Mon, 02 Jan 2006 15:04:05 -0700"
		time.RFC1123,           // "Mon, 02 Jan 2006 15:04:05 MST"
		"2006-01-02T15:04:05Z", // ISO 8601 UTC
		"2006-01-02T15:04:05-07:00",
		"2006-01-02 15:04:05",
		"Mon, 2 Jan 2006 15:04:05 -0700",
		"Mon, 2 Jan 2006 15:04:05 MST",
	}
	for _, layout := range layouts {
		if t, err := time.Parse(layout, s); err == nil {
			return t, nil
		}
	}
	return time.Time{}, fmt.Errorf("unrecognised date format: %q", s)
}

// cleanDescription strips HTML tags and trims whitespace from a description
// string so it can be stored as plain text in Details.
func cleanDescription(s string) string {
	// Simple HTML tag removal — adequate for RSS descriptions.
	var out strings.Builder
	inTag := false
	for _, r := range s {
		switch {
		case r == '<':
			inTag = true
		case r == '>':
			inTag = false
		case !inTag:
			out.WriteRune(r)
		}
	}
	result := strings.Join(strings.Fields(out.String()), " ")
	return truncate(result, 200)
}
