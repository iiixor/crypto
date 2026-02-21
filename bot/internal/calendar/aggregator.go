package calendar

import (
	"context"
	"encoding/json"
	"log"
	"os"
	"sync"
	"time"

	"crypto-bot/internal/model"
	"crypto-bot/internal/scanner"
)

// Scanner — интерфейс для всех источников данных
type Scanner interface {
	Scan(ctx context.Context) ([]model.Event, error)
}

// Aggregator собирает события из всех источников и хранит кэш в events.json
type Aggregator struct {
	scanners  []Scanner
	cachePath string
	mu        sync.Mutex
	cache     map[string]model.Event // id → event
}

func NewAggregator(cachePath string, sources struct {
	Binance      bool
	Bybit        bool
	OKX          bool
	TokenUnlocks bool
	Airdrops     bool
}) *Aggregator {
	a := &Aggregator{
		cachePath: cachePath,
		cache:     make(map[string]model.Event),
	}

	if sources.Binance {
		a.scanners = append(a.scanners, scanner.NewBinanceScanner())
	}
	if sources.Bybit {
		a.scanners = append(a.scanners, scanner.NewBybitScanner())
	}
	if sources.OKX {
		a.scanners = append(a.scanners, scanner.NewOKXScanner())
	}
	if sources.TokenUnlocks {
		a.scanners = append(a.scanners, scanner.NewUnlocksScanner())
	}
	if sources.Airdrops {
		a.scanners = append(a.scanners, scanner.NewAirdropsScanner())
	}

	// Загружаем кэш с диска при старте
	a.loadCache()
	return a
}

// Refresh опрашивает все источники, обновляет кэш, возвращает список всех событий
func (a *Aggregator) Refresh(ctx context.Context) []model.Event {
	// Параллельный сбор со всех источников
	type result struct {
		events []model.Event
	}
	ch := make(chan result, len(a.scanners))

	for _, s := range a.scanners {
		s := s
		go func() {
			scanCtx, cancel := context.WithTimeout(ctx, 20*time.Second)
			defer cancel()
			evs, err := s.Scan(scanCtx)
			if err != nil {
				log.Printf("[aggregator] scanner error: %v", err)
				evs = nil
			}
			ch <- result{events: evs}
		}()
	}

	var fresh []model.Event
	for range a.scanners {
		r := <-ch
		fresh = append(fresh, r.events...)
	}

	fresh = deduplicateCrossSource(fresh)

	a.mu.Lock()
	defer a.mu.Unlock()

	// Добавляем новые события, сохраняем флаги отправки для существующих
	for _, e := range fresh {
		if existing, ok := a.cache[e.ID]; ok {
			// Обновляем данные, но сохраняем флаги отправки
			e.SentDigest = existing.SentDigest
			e.Sent24h = existing.Sent24h
			e.Sent2h = existing.Sent2h
		}
		a.cache[e.ID] = e
	}

	// Удаляем устаревшие записи из других источников для тех же событий.
	// После deduplicateCrossSource в fresh — только winners. Все записи кэша
	// с тем же TOKEN/DATE/TYPE но другим source — проигравшие, удаляем их.
	for _, winner := range fresh {
		winDate := winner.Date.UTC().Format("20060102")
		for id, cached := range a.cache {
			if id == winner.ID {
				continue
			}
			if cached.Token == winner.Token &&
				cached.Date.UTC().Format("20060102") == winDate &&
				cached.Type == winner.Type {
				delete(a.cache, id)
			}
		}
	}

	// Чистим старые события (старше 2 дней)
	cutoff := time.Now().UTC().Add(-48 * time.Hour)
	for id, e := range a.cache {
		if e.Date.Before(cutoff) {
			delete(a.cache, id)
		}
	}

	a.saveCache()
	return a.allEvents()
}

// Events возвращает текущий кэш без запроса источников
func (a *Aggregator) Events() []model.Event {
	a.mu.Lock()
	defer a.mu.Unlock()
	return deduplicateCrossSource(a.allEvents())
}

// MarkSentDigest помечает событие как отправленное в дайджест
func (a *Aggregator) MarkSentDigest(id string) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if e, ok := a.cache[id]; ok {
		e.SentDigest = true
		a.cache[id] = e
	}
	a.saveCache()
}

// MarkSent24h помечает событие как отправленное (алерт 24ч)
func (a *Aggregator) MarkSent24h(id string) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if e, ok := a.cache[id]; ok {
		e.Sent24h = true
		a.cache[id] = e
	}
	a.saveCache()
}

// MarkSent2h помечает событие как отправленное (алерт 2ч)
func (a *Aggregator) MarkSent2h(id string) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if e, ok := a.cache[id]; ok {
		e.Sent2h = true
		a.cache[id] = e
	}
	a.saveCache()
}

// sourcePriority returns a lower number for higher-priority sources.
func sourcePriority(source string) int {
	switch source {
	case "binance":
		return 1
	case "bybit":
		return 2
	case "okx":
		return 3
	case "tokenunlocks":
		return 4
	case "airdrops":
		return 5
	}
	return 99
}

// deduplicateCrossSource removes duplicate events from different sources.
// When the same TOKEN+DATE+TYPE appears from multiple sources, the highest-priority
// source wins. Sent flags are preserved from whichever entry had them set.
func deduplicateCrossSource(events []model.Event) []model.Event {
	type key struct {
		token string
		date  string
		eType model.EventType
	}

	// Group events by cross-source key
	groups := make(map[key][]model.Event)
	var order []key // preserve insertion order for deterministic output
	for _, e := range events {
		k := key{
			token: e.Token,
			date:  e.Date.UTC().Format("20060102"),
			eType: e.Type,
		}
		if _, exists := groups[k]; !exists {
			order = append(order, k)
		}
		groups[k] = append(groups[k], e)
	}

	out := make([]model.Event, 0, len(order))
	for _, k := range order {
		group := groups[k]
		if len(group) == 1 {
			out = append(out, group[0])
			continue
		}
		// Pick winner: lowest priority number = highest priority
		winner := group[0]
		for _, e := range group[1:] {
			if sourcePriority(e.Source) < sourcePriority(winner.Source) {
				winner = e
			}
		}
		// Merge sent flags from all group members
		for _, e := range group {
			if e.SentDigest {
				winner.SentDigest = true
			}
			if e.Sent24h {
				winner.Sent24h = true
			}
			if e.Sent2h {
				winner.Sent2h = true
			}
		}
		out = append(out, winner)
	}
	return out
}

func (a *Aggregator) allEvents() []model.Event {
	out := make([]model.Event, 0, len(a.cache))
	for _, e := range a.cache {
		out = append(out, e)
	}
	return out
}

func (a *Aggregator) loadCache() {
	data, err := os.ReadFile(a.cachePath)
	if err != nil {
		// Файл может не существовать при первом запуске
		return
	}
	var events []model.Event
	if err := json.Unmarshal(data, &events); err != nil {
		log.Printf("[aggregator] failed to parse cache: %v", err)
		return
	}
	for _, e := range events {
		a.cache[e.ID] = e
	}
	log.Printf("[aggregator] loaded %d events from cache", len(a.cache))
}

func (a *Aggregator) saveCache() {
	events := a.allEvents()
	data, err := json.MarshalIndent(events, "", "  ")
	if err != nil {
		log.Printf("[aggregator] failed to marshal cache: %v", err)
		return
	}
	if err := os.WriteFile(a.cachePath, data, 0644); err != nil {
		log.Printf("[aggregator] failed to save cache: %v", err)
	}
}
