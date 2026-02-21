// scantest — временный инструмент для проверки сканеров
package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"crypto-bot/internal/scanner"
)

func main() {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	scanners := map[string]interface {
		Scan(context.Context) (interface{}, error)
	}{}
	_ = scanners

	run("Binance", func() {
		s := scanner.NewBinanceScanner()
		events, err := s.Scan(ctx)
		if err != nil {
			log.Printf("  ERROR: %v", err)
			return
		}
		fmt.Printf("  Найдено %d событий\n", len(events))
		for _, e := range events {
			fmt.Printf("  [%s] %s — %s (%s)\n", e.Type, e.Token, e.Title, e.Date.Format("02 Jan 15:04 UTC"))
		}
	})

	run("Bybit", func() {
		s := scanner.NewBybitScanner()
		events, err := s.Scan(ctx)
		if err != nil {
			log.Printf("  ERROR: %v", err)
			return
		}
		fmt.Printf("  Найдено %d событий\n", len(events))
		for _, e := range events {
			fmt.Printf("  [%s] %s — %s (%s)\n", e.Type, e.Token, e.Title, e.Date.Format("02 Jan 15:04 UTC"))
		}
	})

	run("OKX", func() {
		s := scanner.NewOKXScanner()
		events, err := s.Scan(ctx)
		if err != nil {
			log.Printf("  ERROR: %v", err)
			return
		}
		fmt.Printf("  Найдено %d событий\n", len(events))
		for _, e := range events {
			fmt.Printf("  [%s] %s — %s (%s)\n", e.Type, e.Token, e.Title, e.Date.Format("02 Jan 15:04 UTC"))
		}
	})

	run("TokenUnlocks", func() {
		s := scanner.NewUnlocksScanner()
		events, err := s.Scan(ctx)
		if err != nil {
			log.Printf("  ERROR: %v", err)
			return
		}
		fmt.Printf("  Найдено %d событий\n", len(events))
		for _, e := range events {
			fmt.Printf("  [%s] %s — %s (%s)\n", e.Type, e.Token, e.Title, e.Date.Format("02 Jan 15:04 UTC"))
		}
	})

	run("Airdrops", func() {
		s := scanner.NewAirdropsScanner()
		events, err := s.Scan(ctx)
		if err != nil {
			log.Printf("  ERROR: %v", err)
			return
		}
		fmt.Printf("  Найдено %d событий\n", len(events))
		for _, e := range events {
			fmt.Printf("  [%s] %s — %s (%s)\n", e.Type, e.Token, e.Title, e.Date.Format("02 Jan 15:04 UTC"))
		}
	})
}

func run(name string, fn func()) {
	fmt.Printf("\n=== %s ===\n", name)
	fn()
}
