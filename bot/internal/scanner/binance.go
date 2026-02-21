package scanner

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"regexp"
	"strings"
	"time"

	"crypto-bot/internal/model"
)

const (
	binanceSource      = "binance"
	binanceUserAgent   = "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36"
	binanceListingURL  = "https://www.binance.com/bapi/composite/v1/public/cms/article/list/query?type=1&pageNo=1&pageSize=20&catalogId=48"
	binanceLaunchURL   = "https://www.binance.com/bapi/composite/v1/public/cms/article/list/query?type=1&pageNo=1&pageSize=20&catalogId=161"
	binanceArticleBase = "https://www.binance.com/en/support/announcement/"
)

// Ищем дату в конце заголовка вида "(2026-02-21)"
var binanceTitleDateRe = regexp.MustCompile(`\((\d{4}-\d{2}-\d{2})\)`)

type binanceResponse struct {
	Data struct {
		// API возвращает articles вложенными в catalogs[0]
		Catalogs []struct {
			Articles []binanceArticle `json:"articles"`
		} `json:"catalogs"`
		// Иногда бывает напрямую
		Articles []binanceArticle `json:"articles"`
	} `json:"data"`
}

type binanceArticle struct {
	ID          int64  `json:"id"`
	Code        string `json:"code"`
	Title       string `json:"title"`
	ReleaseDate int64  `json:"releaseDate"` // milliseconds UTC
}

type BinanceScanner struct {
	client *http.Client
}

func NewBinanceScanner() *BinanceScanner {
	return &BinanceScanner{
		client: &http.Client{Timeout: 15 * time.Second},
	}
}

func (s *BinanceScanner) Scan(ctx context.Context) ([]model.Event, error) {
	// Окно: анонсы за последние 14 дней (биржи анонсируют за 7-14 дней) и на 7 дней вперёд
	from := time.Now().UTC().Add(-14 * 24 * time.Hour)
	to := time.Now().UTC().Add(7 * 24 * time.Hour)

	var events []model.Event
	for _, endpoint := range []string{binanceListingURL, binanceLaunchURL} {
		articles, err := s.fetchArticles(ctx, endpoint)
		if err != nil {
			log.Printf("[binance] warning: %v", err)
			continue
		}
		for _, a := range articles {
			ev, ok := s.parseArticle(a, from, to)
			if !ok {
				continue
			}
			events = append(events, ev)
		}
	}
	return deduplicateEvents(events), nil
}

func (s *BinanceScanner) fetchArticles(ctx context.Context, url string) ([]binanceArticle, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("User-Agent", binanceUserAgent)
	req.Header.Set("Accept", "application/json")
	// НЕ ставим Accept-Encoding: gzip — Go http.Client сам обрабатывает сжатие

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("do request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("status %d", resp.StatusCode)
	}

	var parsed binanceResponse
	if err := json.NewDecoder(resp.Body).Decode(&parsed); err != nil {
		return nil, fmt.Errorf("decode: %w", err)
	}
	// Собираем статьи из всех каталогов
	var articles []binanceArticle
	articles = append(articles, parsed.Data.Articles...)
	for _, cat := range parsed.Data.Catalogs {
		articles = append(articles, cat.Articles...)
	}
	return articles, nil
}

func (s *BinanceScanner) parseArticle(a binanceArticle, from, to time.Time) (model.Event, bool) {
	// releaseDate — дата публикации анонса (мс)
	announceDate := time.Unix(a.ReleaseDate/1000, 0).UTC()

	// Пробуем извлечь дату события из заголовка (формат "(2026-02-21)")
	eventDate := announceDate
	if m := binanceTitleDateRe.FindStringSubmatch(a.Title); len(m) == 2 {
		if parsed, err := time.Parse("2006-01-02", m[1]); err == nil {
			eventDate = parsed.UTC()
		}
	}

	// Включаем если дата анонса или дата события попадает в окно
	if announceDate.Before(from) && eventDate.Before(from) {
		return model.Event{}, false
	}
	if announceDate.After(to) && eventDate.After(to) {
		return model.Event{}, false
	}

	eventType, ok := classifyBinanceTitle(a.Title)
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
		ID:     makeEventID(binanceSource, token, eventDate),
		Type:   eventType,
		Source: binanceSource,
		Token:  strings.ToUpper(token),
		Title:  a.Title,
		Date:   eventDate,
		URL:    binanceArticleBase + a.Code,
	}, true
}

func classifyBinanceTitle(title string) (model.EventType, bool) {
	upper := strings.ToUpper(title)
	// Исключаем делистинги, уведомления, апдейты
	if strings.Contains(upper, "DELIST") || strings.Contains(upper, "REMOVAL") ||
		strings.Contains(upper, "NOTICE ON") || strings.Contains(upper, "SUSPEND") {
		return "", false
	}
	switch {
	case strings.Contains(upper, "LAUNCHPOOL"):
		return model.EventLaunchpool, true
	case strings.Contains(upper, "WILL LIST") || strings.Contains(upper, "WILL LAUNCH") ||
		strings.Contains(upper, "NEW LISTING") || strings.Contains(upper, "PERPETUAL"):
		return model.EventListing, true
	default:
		return "", false
	}
}
