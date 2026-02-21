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
	okxSource = "okx"
	// Официальный API OKX анонсов: тип "announcements-new-listings"
	okxURL = "https://www.okx.com/api/v5/support/announcements?page=1&limit=20&annType=announcements-new-listings"
)

// okxResponse — обёртка ответа OKX API
type okxResponse struct {
	Code string `json:"code"`
	Data []struct {
		Details []okxDetail `json:"details"`
	} `json:"data"`
}

type okxDetail struct {
	AnnType      string `json:"annType"`
	Title        string `json:"title"`
	URL          string `json:"url"`
	PTime        string `json:"pTime"`        // ms UTC как строка
	BusinessPTime string `json:"businessPTime"` // ms UTC как строка
}

type OKXScanner struct {
	client *http.Client
}

func NewOKXScanner() *OKXScanner {
	return &OKXScanner{
		client: &http.Client{Timeout: 15 * time.Second},
	}
}

func (s *OKXScanner) Scan(ctx context.Context) ([]model.Event, error) {
	from := time.Now().UTC().Add(-14 * 24 * time.Hour)
	to := time.Now().UTC().Add(7 * 24 * time.Hour)

	details, err := s.fetchDetails(ctx)
	if err != nil {
		log.Printf("[okx] warning: %v", err)
		return []model.Event{}, nil
	}

	var events []model.Event
	for _, d := range details {
		ev, ok := s.parseDetail(d, from, to)
		if !ok {
			continue
		}
		events = append(events, ev)
	}
	return events, nil
}

func (s *OKXScanner) fetchDetails(ctx context.Context) ([]okxDetail, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, okxURL, nil)
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", binanceUserAgent)

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("do request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("status %d from okx", resp.StatusCode)
	}

	var parsed okxResponse
	if err := json.NewDecoder(resp.Body).Decode(&parsed); err != nil {
		return nil, fmt.Errorf("decode: %w", err)
	}
	if parsed.Code != "0" {
		return nil, fmt.Errorf("okx api error code: %s", parsed.Code)
	}

	// Разворачиваем вложенный список
	var all []okxDetail
	for _, group := range parsed.Data {
		all = append(all, group.Details...)
	}
	return all, nil
}

func (s *OKXScanner) parseDetail(d okxDetail, from, to time.Time) (model.Event, bool) {
	// pTime — миллисекунды UTC как строка
	ms := parseMillisString(d.PTime)
	if ms == 0 {
		ms = parseMillisString(d.BusinessPTime)
	}
	if ms == 0 {
		return model.Event{}, false
	}
	pubDate := time.Unix(ms/1000, 0).UTC()

	eventDate, _ := extractEventDateFromTitle(d.Title, pubDate)

	if eventDate.Before(from) || eventDate.After(to) {
		return model.Event{}, false
	}

	eventType, ok := classifyOKXTitle(d.Title)
	if !ok {
		return model.Event{}, false
	}

	token := extractTokenFromParentheses(d.Title)
	if token == "" {
		token = extractTokenFromTitle(d.Title)
	}
	if token == "" {
		token = "UNKNOWN"
	}

	return model.Event{
		ID:     makeEventID(okxSource, token, eventDate),
		Type:   eventType,
		Source: okxSource,
		Token:  strings.ToUpper(token),
		Title:  d.Title,
		Date:   eventDate,
		URL:    d.URL,
	}, true
}

func classifyOKXTitle(title string) (model.EventType, bool) {
	upper := strings.ToUpper(title)
	switch {
	case strings.Contains(upper, "JUMPSTART"):
		return model.EventLaunchpool, true
	case strings.Contains(upper, "TO LIST") || strings.Contains(upper, "WILL LIST") ||
		strings.Contains(upper, "LISTING") || strings.Contains(upper, "TO SUPPORT"):
		return model.EventListing, true
	default:
		return "", false
	}
}

// parseMillisString разбирает строку с Unix timestamp в миллисекундах
func parseMillisString(s string) int64 {
	if s == "" {
		return 0
	}
	var n int64
	for _, c := range s {
		if c < '0' || c > '9' {
			return 0
		}
		n = n*10 + int64(c-'0')
	}
	return n
}
