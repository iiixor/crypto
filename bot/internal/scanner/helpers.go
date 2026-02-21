package scanner

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"crypto-bot/internal/model"
)

// reISODate matches ISO dates like "2026-02-21" optionally followed by a time "10:00" and "UTC"
var reISODate = regexp.MustCompile(`\b(\d{4}-\d{2}-\d{2})\b(?:\s+(\d{1,2}:\d{2})(?:\s*UTC)?)?`)

// reEnglishDate matches English month names like "February 21" or "Feb 21"
var reEnglishDate = regexp.MustCompile(`\b(January|February|March|April|May|June|July|August|September|October|November|December|Jan|Feb|Mar|Apr|May|Jun|Jul|Aug|Sep|Oct|Nov|Dec)\s+(\d{1,2})\b`)

var monthNames = map[string]time.Month{
	"january": time.January, "jan": time.January,
	"february": time.February, "feb": time.February,
	"march": time.March, "mar": time.March,
	"april": time.April, "apr": time.April,
	"may": time.May,
	"june": time.June, "jun": time.June,
	"july": time.July, "jul": time.July,
	"august": time.August, "aug": time.August,
	"september": time.September, "sep": time.September,
	"october": time.October, "oct": time.October,
	"november": time.November, "nov": time.November,
	"december": time.December, "dec": time.December,
}

// extractEventDateFromTitle tries to extract the actual event date from the title text.
// If nothing is found or the extracted date is more than 7 days before pubDate, it returns (fallback, false).
func extractEventDateFromTitle(title string, fallback time.Time) (time.Time, bool) {
	// Pattern 1: ISO date (2026-02-21) optionally with time 10:00 [UTC]
	if m := reISODate.FindStringSubmatch(title); len(m) >= 2 {
		dateStr := m[1]
		t, err := time.Parse("2006-01-02", dateStr)
		if err == nil {
			if len(m) >= 3 && m[2] != "" {
				parts := strings.Split(m[2], ":")
				if len(parts) == 2 {
					h, _ := strconv.Atoi(parts[0])
					min, _ := strconv.Atoi(parts[1])
					t = time.Date(t.Year(), t.Month(), t.Day(), h, min, 0, 0, time.UTC)
				}
			} else {
				t = time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC)
			}
			if !t.Before(fallback.Add(-7 * 24 * time.Hour)) {
				return t, true
			}
		}
	}

	// Pattern 2: English month name "February 21" / "Feb 21"
	if m := reEnglishDate.FindStringSubmatch(title); len(m) >= 3 {
		monthStr := strings.ToLower(m[1])
		month, ok := monthNames[monthStr]
		if ok {
			day, err := strconv.Atoi(m[2])
			if err == nil {
				year := fallback.Year()
				t := time.Date(year, month, day, 0, 0, 0, 0, time.UTC)
				if !t.Before(fallback.Add(-7 * 24 * time.Hour)) {
					return t, true
				}
			}
		}
	}

	return fallback, false
}

// reParentheses matches the first parenthesised token ticker in an announcement
// e.g. "Bybit Will List BIRB (BIRB)" → "BIRB". Excludes years like (2026-02-21).
var reParentheses = regexp.MustCompile(`\(([A-Z][A-Z0-9]{1,9})\)`)

// reUSDTPair matches "OPNUSDT", "AZTECUSDT" etc. — берём часть до USDT
var reUSDTPair = regexp.MustCompile(`\b([A-Z]{2,10})USDT\b`)

// reBTCPair matches "XYZBTC" pairs
var reBTCPair = regexp.MustCompile(`\b([A-Z]{2,10})BTC\b`)

// reUpperWord matches the first stand-alone ALL-CAPS word of 2–10 letters
var reUpperWord = regexp.MustCompile(`\b([A-Z]{2,10})\b`)

// stopWords — слова которые не являются тикерами токенов
var stopWords = map[string]bool{
	"WILL": true, "LIST": true, "LISTS": true, "THE": true,
	"AND": true, "FOR": true, "WITH": true, "FROM": true,
	"NEW": true, "NOW": true, "OKX": true, "BYBIT": true,
	"BINANCE": true, "SPOT": true, "ZONE": true, "IN": true,
	"ON": true, "OF": true, "TO": true, "IS": true,
	"TGE": true, "IEO": true, "ICO": true, "LAUNCHPOOL": true,
	"JUMPSTART": true, "INNOVATION": true, "TRADING": true,
	"PAIRS": true, "PAIR": true, "MARGIN": true, "FUTURES": true,
	"PRE": true, "MARKET": true, "CONTRACT": true, "PERPETUAL": true,
	"LAUNCH": true, "MULTIPLE": true, "MARGINED": true, "USD": true,
	"NOTICE": true, "REMOVAL": true, "BSC": true,
	"EEA": true, "SUPPORT": true, "CONVERT": true, "STANDARD": true,
	"UP": true, "LEVERAGE": true, "MAIN": true, "EARN": true,
}

// makeEventID produces a deterministic, collision-resistant event identifier.
// Format: "source:TOKEN:YYYYMMDD"
func makeEventID(source, token string, date time.Time) string {
	return fmt.Sprintf("%s:%s:%s",
		source,
		strings.ToUpper(token),
		date.UTC().Format("20060102"),
	)
}

// extractTokenFromParentheses returns the first ALL-CAPS ticker found inside
// parentheses, e.g. "(BIRB)" → "BIRB". Returns "" when not found.
func extractTokenFromParentheses(title string) string {
	m := reParentheses.FindStringSubmatch(title)
	if len(m) < 2 {
		return ""
	}
	return m[1]
}

// extractTokenFromTitle пытается извлечь тикер токена из заголовка анонса.
// Порядок: (TICKER) → XYZUSDT пара → XYZBTC пара → первое незапрещённое слово
func extractTokenFromTitle(title string) string {
	// 1. Ищем (TICKER) — самый надёжный способ
	if tok := extractTokenFromParentheses(title); tok != "" {
		return tok
	}

	// 2. Ищем пару вида XYZUSDT → XYZ
	if m := reUSDTPair.FindStringSubmatch(title); len(m) >= 2 {
		if !stopWords[m[1]] {
			return m[1]
		}
	}

	// 3. Ищем пару вида XYZBTC → XYZ
	if m := reBTCPair.FindStringSubmatch(title); len(m) >= 2 {
		if !stopWords[m[1]] {
			return m[1]
		}
	}

	// 4. Fallback: первое ALL-CAPS слово не из стоп-листа
	matches := reUpperWord.FindAllString(title, -1)
	for _, m := range matches {
		if !stopWords[m] && len(m) >= 2 && len(m) <= 10 {
			return m
		}
	}

	return ""
}

// deduplicateEvents removes events with duplicate IDs, keeping the first
// occurrence. This is used when multiple API endpoints can return the same
// event (e.g. Binance listing + launchpool feeds both mention the same token).
func deduplicateEvents(events []model.Event) []model.Event {
	seen := make(map[string]struct{}, len(events))
	out := make([]model.Event, 0, len(events))
	for _, e := range events {
		if _, exists := seen[e.ID]; exists {
			continue
		}
		seen[e.ID] = struct{}{}
		out = append(out, e)
	}
	return out
}

// truncate shortens s to at most maxRunes runes, appending "…" when cut.
func truncate(s string, maxRunes int) string {
	runes := []rune(s)
	if len(runes) <= maxRunes {
		return s
	}
	return string(runes[:maxRunes-1]) + "…"
}
