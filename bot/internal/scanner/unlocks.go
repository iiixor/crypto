package scanner

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"math"
	"net/http"
	"strings"
	"time"

	"crypto-bot/internal/model"
)

const (
	unlocksSource = "tokenunlocks"
	// tokenUnlocksURL — публичный эндпоинт разлоков от token.unlocks.app
	tokenUnlocksURL = "https://token.unlocks.app/api/v1/upcoming?days=7"
	// tokenUnlocksURL2 — резервный эндпоинт
	tokenUnlocksURL2 = "https://tokenunlocks.app/api/unlocks?days=7"
)

// unlockEvent represents a single token unlock from the tokenunlocks.app API.
type unlockEvent struct {
	Token          string  `json:"token"`
	Name           string  `json:"name"`
	UnlockDate     string  `json:"unlockDate"`     // "2026-02-20"
	UnlockPercent  float64 `json:"unlockPercent"`  // percentage of total supply
	UnlockValueUSD float64 `json:"unlockValueUSD"` // approximate USD value
}

// UnlocksScanner fetches upcoming token unlock events from tokenunlocks.app.
type UnlocksScanner struct {
	client *http.Client
}

// NewUnlocksScanner constructs an UnlocksScanner with a sensible HTTP client.
func NewUnlocksScanner() *UnlocksScanner {
	return &UnlocksScanner{
		client: &http.Client{Timeout: 15 * time.Second},
	}
}

// Scan fetches token unlock events and returns those within the next 7 days.
// If the upstream API is unavailable the scanner logs a warning and returns an
// empty (non-nil) slice — callers should treat this as a graceful degradation.
func (s *UnlocksScanner) Scan(ctx context.Context) ([]model.Event, error) {
	now := time.Now().UTC()
	horizon := now.Add(7 * 24 * time.Hour)

	unlocks, err := s.fetchUnlocks(ctx)
	if err != nil {
		log.Printf("[tokenunlocks] warning: failed to fetch unlocks: %v", err)
		return []model.Event{}, nil
	}

	var events []model.Event
	for _, u := range unlocks {
		ev, ok := parseUnlock(u, now, horizon)
		if !ok {
			continue
		}
		events = append(events, ev)
	}

	return events, nil
}

// fetchUnlocks performs the HTTP GET and decodes the tokenunlocks.app response.
func (s *UnlocksScanner) fetchUnlocks(ctx context.Context) ([]unlockEvent, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, tokenUnlocksURL, nil)
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
		return nil, fmt.Errorf("unexpected status %d from tokenunlocks", resp.StatusCode)
	}

	var unlocks []unlockEvent
	if err := json.NewDecoder(resp.Body).Decode(&unlocks); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	return unlocks, nil
}

// parseUnlock converts a raw unlock entry into a model.Event.
// Returns (event, false) when the entry should be skipped.
func parseUnlock(u unlockEvent, now, horizon time.Time) (model.Event, bool) {
	if u.UnlockDate == "" {
		return model.Event{}, false
	}

	// UnlockDate is a plain date string "YYYY-MM-DD"; treat as UTC midnight.
	eventDate, err := time.ParseInLocation("2006-01-02", u.UnlockDate, time.UTC)
	if err != nil {
		log.Printf("[tokenunlocks] warning: cannot parse unlockDate %q: %v", u.UnlockDate, err)
		return model.Event{}, false
	}

	if eventDate.Before(now) || eventDate.After(horizon) {
		return model.Event{}, false
	}

	token := strings.ToUpper(u.Token)
	if token == "" {
		token = "UNKNOWN"
	}

	name := u.Name
	if name == "" {
		name = token
	}

	details := formatUnlockDetails(u.UnlockPercent, u.UnlockValueUSD)

	return model.Event{
		ID:      makeEventID(unlocksSource, token, eventDate),
		Type:    model.EventUnlock,
		Source:  unlocksSource,
		Token:   token,
		Title:   fmt.Sprintf("%s (%s) — разлок токенов", name, token),
		Date:    eventDate,
		URL:     fmt.Sprintf("https://tokenunlocks.app/token/%s", strings.ToLower(token)),
		Details: details,
	}, true
}

// formatUnlockDetails builds a human-readable details string for an unlock.
// Example: "разлок 15% supply (~$120M)"
func formatUnlockDetails(pct, valueUSD float64) string {
	if pct <= 0 && valueUSD <= 0 {
		return ""
	}

	var parts []string

	if pct > 0 {
		// Format percentage: show one decimal place only when meaningful.
		if pct == math.Trunc(pct) {
			parts = append(parts, fmt.Sprintf("разлок %.0f%% supply", pct))
		} else {
			parts = append(parts, fmt.Sprintf("разлок %.1f%% supply", pct))
		}
	}

	if valueUSD > 0 {
		millions := valueUSD / 1_000_000
		if millions >= 1 {
			parts = append(parts, fmt.Sprintf("~$%.0fM", millions))
		} else {
			thousands := valueUSD / 1_000
			parts = append(parts, fmt.Sprintf("~$%.0fK", thousands))
		}
	}

	return strings.Join(parts, " ")
}
