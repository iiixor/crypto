// digest — немедленно отправляет дайджест в Telegram для проверки
package main

import (
	"context"
	"flag"
	"log"
	"time"

	"crypto-bot/internal/calendar"
	"crypto-bot/internal/config"
	"crypto-bot/internal/notify"
)

func main() {
	cfgPath := flag.String("config", "config.yaml", "path to config file")
	flag.Parse()

	cfg, err := config.Load(*cfgPath)
	if err != nil {
		log.Fatalf("load config: %v", err)
	}

	tg := notify.NewTelegram(cfg.Telegram.BotToken, cfg.Telegram.ChatID)

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

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	log.Println("Собираем события...")
	events := agg.Refresh(ctx)
	log.Printf("Получено %d событий", len(events))

	digestEvents := calendar.EventsForDigest(agg.Events())
	log.Printf("События для дайджеста: %d", len(digestEvents))

	now := time.Now().UTC()
	msg := notify.FormatDigest(digestEvents, now, now.Add(7*24*time.Hour))

	log.Println("Отправляем в Telegram...")
	if err := tg.Send(msg); err != nil {
		log.Fatalf("send error: %v", err)
	}

	for _, e := range digestEvents {
		agg.MarkSentDigest(e.ID)
	}
	log.Println("Дайджест отправлен!")
}
