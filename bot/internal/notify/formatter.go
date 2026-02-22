package notify

import (
	"fmt"
	"strings"
	"time"

	"crypto-bot/internal/model"
)

// escMD2 экранирует специальные символы для Telegram MarkdownV2
func escMD2(s string) string {
	// Символы, которые нужно экранировать в MarkdownV2:
	// _ * [ ] ( ) ~ ` > # + - = | { } . !
	var replacer = strings.NewReplacer(
		`_`, `\_`, `*`, `\*`, `[`, `\[`, `]`, `\]`,
		`(`, `\(`, `)`, `\)`, `~`, `\~`, "`", "\\`",
		`>`, `\>`, `#`, `\#`, `+`, `\+`, `-`, `\-`,
		`=`, `\=`, `|`, `\|`, `{`, `\{`, `}`, `\}`,
		`.`, `\.`, `!`, `\!`,
	)
	return replacer.Replace(s)
}

const separator = "——————————————————"

// FormatDigest формирует понедельничный дайджест на неделю
func FormatDigest(events []model.Event, weekStart, weekEnd time.Time) string {
	var sb strings.Builder

	startStr := weekStart.Format("02 Jan")
	endStr := weekEnd.Format("02 Jan 2006")
	sb.WriteString(fmt.Sprintf("📅 *СОБЫТИЯ НЕДЕЛИ*\n_%s — %s_\n",
		escMD2(startStr), escMD2(endStr)))

	launchpools := filterByType(events, model.EventLaunchpool)
	listings := filterByType(events, model.EventListing)
	unlocks := filterByType(events, model.EventUnlock)
	airdrops := filterByType(events, model.EventAirdrop)

	if len(launchpools) > 0 {
		sb.WriteString(fmt.Sprintf("\n%s\n", escMD2(separator)))
		sb.WriteString(fmt.Sprintf("🌾 *LAUNCHPOOL* \\(%d\\)\n\n", len(launchpools)))
		for _, e := range launchpools {
			writeDigestEvent(&sb, e)
		}
	}

	if len(listings) > 0 {
		sb.WriteString(fmt.Sprintf("\n%s\n", escMD2(separator)))
		sb.WriteString(fmt.Sprintf("🆕 *ЛИСТИНГИ* \\(%d\\)\n\n", len(listings)))
		for _, e := range listings {
			writeDigestEvent(&sb, e)
		}
	}

	if len(unlocks) > 0 {
		sb.WriteString(fmt.Sprintf("\n%s\n", escMD2(separator)))
		sb.WriteString(fmt.Sprintf("🔓 *РАЗЛОКИ* \\(%d\\)\n\n", len(unlocks)))
		for _, e := range unlocks {
			writeDigestEvent(&sb, e)
		}
	}

	if len(airdrops) > 0 {
		sb.WriteString(fmt.Sprintf("\n%s\n", escMD2(separator)))
		sb.WriteString(fmt.Sprintf("🪂 *TGE / AIRDROP* \\(%d\\)\n\n", len(airdrops)))
		for _, e := range airdrops {
			writeDigestEvent(&sb, e)
		}
	}

	sb.WriteString(fmt.Sprintf("\n%s\n", escMD2(separator)))
	if len(events) == 0 {
		sb.WriteString(escMD2("На этой неделе событий не найдено.") + "\n")
	} else {
		sb.WriteString(fmt.Sprintf("📊 *Итого:* %d %s\n",
			len(events), escMD2(pluralEvents(len(events)))))
		sb.WriteString(escMD2("ℹ️ Алерты придут за 24ч и за 2ч до события") + "\n")
	}

	return sb.String()
}

// writeDigestEvent пишет одно событие в дайджесте
func writeDigestEvent(sb *strings.Builder, e model.Event) {
	sb.WriteString(fmt.Sprintf("▸ *%s* — %s\n",
		escMD2(e.Token), escMD2(capitalize(e.Source))))
	sb.WriteString(fmt.Sprintf("  📅 %s", escMD2(fmtDate(e.Date))))
	if !e.Date.IsZero() && (e.Date.Hour() != 0 || e.Date.Minute() != 0) {
		sb.WriteString(fmt.Sprintf(", %s UTC", escMD2(e.Date.UTC().Format("15:04"))))
	}
	sb.WriteString("\n")
	if e.Details != "" {
		sb.WriteString(fmt.Sprintf("  ℹ️ %s\n", escMD2(e.Details)))
	}
	if e.URL != "" {
		sb.WriteString(fmt.Sprintf("  🔗 [Подробнее](%s)\n", e.URL))
	}
}

// FormatAlert24h формирует алерт за 24 часа до события
func FormatAlert24h(e model.Event) string {
	icon, label, strategy := eventMeta(e)

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("%s *ЗАВТРА \\| %s*\n", icon, escMD2(label)))
	sb.WriteString(fmt.Sprintf("%s\n", escMD2(separator)))
	sb.WriteString("\n")
	sb.WriteString(fmt.Sprintf("*%s* — %s\n", escMD2(e.Token), escMD2(e.Title)))
	sb.WriteString(fmt.Sprintf("📅 %s UTC\n", escMD2(e.Date.UTC().Format("02 Jan 2006, 15:04"))))
	sb.WriteString(fmt.Sprintf("📍 %s\n", escMD2(capitalize(e.Source))))
	sb.WriteString("\n")
	if strategy != "" {
		sb.WriteString(fmt.Sprintf("💡 *Стратегия:* %s\n", escMD2(strategy)))
	}
	if e.URL != "" {
		sb.WriteString(fmt.Sprintf("🔗 [Анонс](%s)\n", e.URL))
	}
	return sb.String()
}

// FormatAlert2h формирует алерт за 2 часа до события
func FormatAlert2h(e model.Event) string {
	_, label, strategy := eventMeta(e)

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("🚨 *ЧЕРЕЗ 2 ЧАСА \\| %s*\n", escMD2(label)))
	sb.WriteString(fmt.Sprintf("%s\n", escMD2(separator)))
	sb.WriteString("\n")
	sb.WriteString(fmt.Sprintf("*%s* запускается в *%s UTC*\n",
		escMD2(e.Token), escMD2(e.Date.UTC().Format("15:04"))))
	sb.WriteString(fmt.Sprintf("📍 %s\n", escMD2(capitalize(e.Source))))
	sb.WriteString("\n")
	if strategy != "" {
		sb.WriteString(fmt.Sprintf("💡 *Стратегия:* %s\n", escMD2(strategy)))
	}
	if e.URL != "" {
		sb.WriteString(fmt.Sprintf("🔗 [Анонс](%s)\n", e.URL))
	}
	return sb.String()
}

func eventMeta(e model.Event) (icon, label, strategy string) {
	switch e.Type {
	case model.EventLaunchpool:
		return "🌾", "LAUNCHPOOL", "Launchpool Harvest (3x, шорт за 12-24ч до листинга)"
	case model.EventListing:
		return "🆕", "ЛИСТИНГ", "Token Splash Short (4x, ждать RSI>85 + Volume>300%)"
	case model.EventUnlock:
		return "🔓", "РАЗЛОК", "VC-Gravity (шорт при разлоке >5% supply)"
	case model.EventAirdrop:
		return "🪂", "AIRDROP/TGE", "TGE Short (4x, продать на спайке первых минут)"
	}
	return "📌", string(e.Type), ""
}

func filterByType(events []model.Event, t model.EventType) []model.Event {
	var out []model.Event
	for _, e := range events {
		if e.Type == t {
			out = append(out, e)
		}
	}
	return out
}

func fmtDate(t time.Time) string {
	months := []string{"", "янв", "фев", "мар", "апр", "май", "июн", "июл", "авг", "сен", "окт", "ноя", "дек"}
	return fmt.Sprintf("%d %s", t.Day(), months[t.Month()])
}

// FormatHelp returns a welcome message with the list of available commands.
func FormatHelp() string {
	var sb strings.Builder
	sb.WriteString("🤖 *Crypto Calendar Bot*\n")
	sb.WriteString(fmt.Sprintf("%s\n\n", escMD2(separator)))

	sb.WriteString("📋 *Просмотр событий:*\n")
	sb.WriteString(escMD2("/today    — события сегодня") + "\n")
	sb.WriteString(escMD2("/tomorrow — события завтра") + "\n")
	sb.WriteString(escMD2("/week     — события на неделю") + "\n")
	sb.WriteString(escMD2("/digest   — дайджест недели") + "\n")

	sb.WriteString("\n🔎 *По категориям:*\n")
	sb.WriteString(escMD2("/listings    — предстоящие листинги") + "\n")
	sb.WriteString(escMD2("/unlocks     — предстоящие разлоки") + "\n")
	sb.WriteString(escMD2("/airdrops    — аирдропы и TGE") + "\n")
	sb.WriteString(escMD2("/launchpools — лаунчпулы") + "\n")

	sb.WriteString("\n⚙️ *Управление:*\n")
	sb.WriteString(escMD2("/refresh — обновить данные") + "\n")

	return sb.String()
}

// FormatEventList formats a list of events with a header for command responses.
func FormatEventList(events []model.Event, header string) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("*%s*\n", escMD2(header)))
	sb.WriteString(fmt.Sprintf("%s\n", escMD2(separator)))

	if len(events) == 0 {
		sb.WriteString("\n" + escMD2("Событий не найдено.") + "\n")
		return sb.String()
	}

	// Группируем по типу, если есть разные типы
	types := []model.EventType{model.EventLaunchpool, model.EventListing, model.EventUnlock, model.EventAirdrop}
	hasMultipleTypes := countTypes(events) > 1

	for _, t := range types {
		group := filterByType(events, t)
		if len(group) == 0 {
			continue
		}

		if hasMultipleTypes {
			icon := eventIcon(t)
			sb.WriteString(fmt.Sprintf("\n%s *%s*\n", icon, escMD2(typeLabelRu(t))))
		} else {
			sb.WriteString("\n")
		}

		for _, e := range group {
			sb.WriteString(fmt.Sprintf("▸ *%s*", escMD2(e.Token)))
			if e.Title != "" && e.Title != e.Token {
				sb.WriteString(fmt.Sprintf(" — %s", escMD2(truncateTitle(e.Title, 80))))
			}
			sb.WriteString("\n")
			sb.WriteString(fmt.Sprintf("  📅 %s UTC", escMD2(e.Date.UTC().Format("02 Jan, 15:04"))))
			sb.WriteString(fmt.Sprintf("  📍 %s\n", escMD2(capitalize(e.Source))))
			if e.Details != "" {
				sb.WriteString(fmt.Sprintf("  ℹ️ %s\n", escMD2(e.Details)))
			}
			if e.URL != "" {
				sb.WriteString(fmt.Sprintf("  🔗 [Подробнее](%s)\n", e.URL))
			}
		}
	}

	sb.WriteString(fmt.Sprintf("\n%s\n", escMD2(separator)))
	sb.WriteString(fmt.Sprintf("📊 %s %s\n",
		escMD2(fmt.Sprintf("%d", len(events))),
		escMD2(pluralEvents(len(events)))))
	return sb.String()
}

func eventIcon(t model.EventType) string {
	switch t {
	case model.EventLaunchpool:
		return "🌾"
	case model.EventListing:
		return "🆕"
	case model.EventUnlock:
		return "🔓"
	case model.EventAirdrop:
		return "🪂"
	}
	return "📌"
}

func typeLabelRu(t model.EventType) string {
	switch t {
	case model.EventLaunchpool:
		return "Launchpool"
	case model.EventListing:
		return "Листинги"
	case model.EventUnlock:
		return "Разлоки"
	case model.EventAirdrop:
		return "TGE / Airdrop"
	}
	return string(t)
}

func countTypes(events []model.Event) int {
	seen := make(map[model.EventType]struct{})
	for _, e := range events {
		seen[e.Type] = struct{}{}
	}
	return len(seen)
}

func capitalize(s string) string {
	switch s {
	case "binance":
		return "Binance"
	case "bybit":
		return "Bybit"
	case "okx":
		return "OKX"
	case "tokenunlocks":
		return "TokenUnlocks"
	case "airdrops":
		return "Airdrops.io"
	}
	if s == "" {
		return s
	}
	return strings.ToUpper(s[:1]) + s[1:]
}

// pluralEvents возвращает правильное склонение слова "событие"
func pluralEvents(n int) string {
	abs := n
	if abs < 0 {
		abs = -abs
	}
	mod10 := abs % 10
	mod100 := abs % 100

	if mod100 >= 11 && mod100 <= 19 {
		return "событий"
	}
	switch mod10 {
	case 1:
		return "событие"
	case 2, 3, 4:
		return "события"
	default:
		return "событий"
	}
}

// truncateTitle обрезает заголовок до maxLen символов
func truncateTitle(s string, maxLen int) string {
	runes := []rune(s)
	if len(runes) <= maxLen {
		return s
	}
	return string(runes[:maxLen-1]) + "…"
}
