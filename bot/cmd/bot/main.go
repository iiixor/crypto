package main

import (
	"context"
	"flag"
	"log"
	"strings"
	"time"

	"crypto-bot/internal/calendar"
	"crypto-bot/internal/config"
	"crypto-bot/internal/notify"
)

func main() {
	cfgPath := flag.String("config", "config.yaml", "path to config file")
	testMode := flag.Bool("test", false, "отправить тестовое сообщение и выйти")
	flag.Parse()

	cfg, err := config.Load(*cfgPath)
	if err != nil {
		log.Fatalf("failed to load config: %v", err)
	}

	if cfg.Telegram.BotToken == "" || cfg.Telegram.ChatID == "" {
		log.Fatal("telegram.bot_token and telegram.chat_id must be set in config.yaml")
	}

	tg := notify.NewTelegram(cfg.Telegram.BotToken, cfg.Telegram.ChatID)

	// Тестовый режим: отправить одно сообщение и выйти
	if *testMode {
		if err := tg.SendPlain("✅ Бот запущен! Crypto Calendar Bot работает."); err != nil {
			log.Fatalf("test send failed: %v", err)
		}
		log.Println("Test message sent successfully")
		return
	}

	agg := calendar.NewAggregator("data/events.json", struct {
		Binance      bool
		Bybit        bool
		OKX          bool
		TokenUnlocks bool
		Airdrops     bool
	}{
		Binance:      cfg.Sources.Binance,
		Bybit:        cfg.Sources.Bybit,
		OKX:          cfg.Sources.OKX,
		TokenUnlocks: cfg.Sources.TokenUnlocks,
		Airdrops:     cfg.Sources.Airdrops,
	})

	log.Println("Crypto Calendar Bot started")
	log.Printf("Refresh interval: %d min", cfg.Scanner.RefreshIntervalMinutes)

	refreshTicker := time.NewTicker(time.Duration(cfg.Scanner.RefreshIntervalMinutes) * time.Minute)
	hourTicker := time.NewTicker(time.Hour)
	defer refreshTicker.Stop()
	defer hourTicker.Stop()

	// Сразу при старте обновляем данные
	ctx := context.Background()
	log.Println("[main] initial data refresh...")
	agg.Refresh(ctx)
	log.Printf("[main] loaded %d events", len(agg.Events()))

	// Удаляем webhook — иначе getUpdates конфликтует с ним и не получает сообщения
	if err := tg.DeleteWebhook(); err != nil {
		log.Printf("[main] deleteWebhook warning: %v", err)
	} else {
		log.Println("[main] webhook deleted (or was not set)")
	}

	// Запускаем обработчик команд Telegram
	handler := notify.NewCommandHandler(tg, agg)
	go startPolling(tg, handler)

	// Проверяем нужно ли отправить дайджест при старте
	checkDigest(tg, agg, cfg.Schedule.DigestWeekday, cfg.Schedule.DigestTimeUTC)

	for {
		select {
		case <-refreshTicker.C:
			log.Println("[main] refreshing data...")
			agg.Refresh(ctx)
			log.Printf("[main] %d events in cache", len(agg.Events()))

		case <-hourTicker.C:
			checkDigest(tg, agg, cfg.Schedule.DigestWeekday, cfg.Schedule.DigestTimeUTC)
			checkAlerts24h(tg, agg)
			checkAlerts2h(tg, agg)
		}
	}
}

// startPolling receives Telegram updates and dispatches commands to the handler.
func startPolling(tg *notify.Telegram, handler *notify.CommandHandler) {
	var offset int64
	for {
		updates, nextOffset, err := tg.GetUpdates(offset, 30)
		if err != nil {
			log.Printf("[polling] error: %v", err)
			time.Sleep(5 * time.Second)
			continue
		}
		offset = nextOffset
		for _, u := range updates {
			if u.Message == nil || !strings.HasPrefix(u.Message.Text, "/") {
				continue
			}
			go handler.Handle(u.Message.Chat.ID, u.Message.Text)
		}
	}
}

// checkDigest отправляет понедельничный дайджест если сейчас нужное время
func checkDigest(tg *notify.Telegram, agg *calendar.Aggregator, weekday, timeUTC string) {
	now := time.Now().UTC()

	targetDay := strings.ToLower(weekday)
	var wantDay time.Weekday
	switch targetDay {
	case "monday":
		wantDay = time.Monday
	case "tuesday":
		wantDay = time.Tuesday
	case "wednesday":
		wantDay = time.Wednesday
	case "thursday":
		wantDay = time.Thursday
	case "friday":
		wantDay = time.Friday
	default:
		wantDay = time.Monday
	}

	if now.Weekday() != wantDay {
		return
	}

	// Парсим целевое время
	parts := strings.Split(timeUTC, ":")
	if len(parts) != 2 {
		return
	}
	var h, m int
	if _, err := parseIntPair(parts[0], parts[1], &h, &m); err != nil {
		return
	}

	// Проверяем: текущий час совпадает с настроенным временем
	if now.Hour() != h {
		return
	}

	events := calendar.EventsForDigest(agg.Events())
	now = time.Now().UTC()
	weekStart := now
	weekEnd := now.Add(7 * 24 * time.Hour)

	msg := notify.FormatDigest(events, weekStart, weekEnd)
	if err := tg.Send(msg); err != nil {
		log.Printf("[digest] send error: %v", err)
		return
	}

	// Помечаем все события как отправленные в дайджест
	for _, e := range events {
		agg.MarkSentDigest(e.ID)
	}
	log.Printf("[digest] sent with %d events", len(events))
}

// checkAlerts24h проверяет события завтра и отправляет алерты
func checkAlerts24h(tg *notify.Telegram, agg *calendar.Aggregator) {
	events := calendar.EventsTomorrow(agg.Events())
	for _, e := range events {
		msg := notify.FormatAlert24h(e)
		if err := tg.Send(msg); err != nil {
			log.Printf("[alert24h] send error for %s: %v", e.ID, err)
			continue
		}
		agg.MarkSent24h(e.ID)
		log.Printf("[alert24h] sent for %s", e.ID)
	}
}

// checkAlerts2h проверяет события через ~2 часа и отправляет алерты
func checkAlerts2h(tg *notify.Telegram, agg *calendar.Aggregator) {
	events := calendar.EventsIn2Hours(agg.Events())
	for _, e := range events {
		msg := notify.FormatAlert2h(e)
		if err := tg.Send(msg); err != nil {
			log.Printf("[alert2h] send error for %s: %v", e.ID, err)
			continue
		}
		agg.MarkSent2h(e.ID)
		log.Printf("[alert2h] sent for %s", e.ID)
	}
}

func parseIntPair(a, b string, x, y *int) (bool, error) {
	var err error
	*x, err = parseInt(a)
	if err != nil {
		return false, err
	}
	*y, err = parseInt(b)
	if err != nil {
		return false, err
	}
	return true, nil
}

func parseInt(s string) (int, error) {
	n := 0
	for _, c := range s {
		if c < '0' || c > '9' {
			return 0, &parseError{s}
		}
		n = n*10 + int(c-'0')
	}
	return n, nil
}

type parseError struct{ s string }

func (e *parseError) Error() string { return "invalid number: " + e.s }
