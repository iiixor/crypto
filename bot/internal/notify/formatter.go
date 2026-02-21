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

// FormatDigest формирует понедельничный дайджест на неделю
func FormatDigest(events []model.Event, weekStart, weekEnd time.Time) string {
	var sb strings.Builder

	startStr := weekStart.Format("2 Jan")
	endStr := weekEnd.Format("2 Jan 2006")
	sb.WriteString(fmt.Sprintf("📅 *СОБЫТИЯ НЕДЕЛИ \\| %s – %s*\n",
		escMD2(startStr), escMD2(endStr)))

	launchpools := filterByType(events, model.EventLaunchpool)
	listings := filterByType(events, model.EventListing)
	unlocks := filterByType(events, model.EventUnlock)
	airdrops := filterByType(events, model.EventAirdrop)

	if len(launchpools) > 0 {
		sb.WriteString("\n🌾 *LAUNCHPOOL*\n")
		for _, e := range launchpools {
			sb.WriteString(fmt.Sprintf("• %s — %s: %s\n",
				escMD2(fmtDate(e.Date)), escMD2(capitalize(e.Source)), escMD2(e.Title)))
		}
	}

	if len(listings) > 0 {
		sb.WriteString("\n🆕 *ЛИСТИНГИ*\n")
		for _, e := range listings {
			line := fmt.Sprintf("• %s — %s: %s",
				escMD2(fmtDate(e.Date)), escMD2(capitalize(e.Source)), escMD2(e.Title))
			if e.Details != "" {
				line += " \\(" + escMD2(e.Details) + "\\)"
			}
			sb.WriteString(line + "\n")
		}
	}

	if len(unlocks) > 0 {
		sb.WriteString("\n🔓 *РАЗЛОКИ \\(VC\\-Gravity триггеры\\)*\n")
		for _, e := range unlocks {
			line := fmt.Sprintf("• %s — %s", escMD2(fmtDate(e.Date)), escMD2(e.Title))
			if e.Details != "" {
				line += ": " + escMD2(e.Details)
			}
			sb.WriteString(line + "\n")
		}
	}

	if len(airdrops) > 0 {
		sb.WriteString("\n🪂 *TGE / AIRDROP*\n")
		for _, e := range airdrops {
			line := fmt.Sprintf("• %s — %s", escMD2(fmtDate(e.Date)), escMD2(e.Title))
			if e.Details != "" {
				line += ", " + escMD2(e.Details)
			}
			sb.WriteString(line + "\n")
		}
	}

	if len(events) == 0 {
		sb.WriteString("\nНа этой неделе событий не найдено\\.\n")
	} else {
		sb.WriteString("\nℹ️ Алерты придут за 24ч и за 2ч до каждого события\n")
	}

	return sb.String()
}

// FormatAlert24h формирует алерт за 24 часа до события
func FormatAlert24h(e model.Event) string {
	icon, label, strategy := eventMeta(e)
	_ = icon
	msg := fmt.Sprintf("⏰ *ЗАВТРА \\| %s*\n", escMD2(label))
	msg += escMD2(e.Title) + "\n"
	msg += fmt.Sprintf("📅 %s UTC\n", escMD2(e.Date.UTC().Format("2 Jan 2006, 15:04")))
	msg += fmt.Sprintf("💡 Стратегия: %s\n", escMD2(strategy))
	if e.URL != "" {
		msg += fmt.Sprintf("🔗 [Анонс](%s)\n", e.URL)
	}
	return msg
}

// FormatAlert2h формирует алерт за 2 часа до события
func FormatAlert2h(e model.Event) string {
	_, label, strategy := eventMeta(e)
	msg := fmt.Sprintf("🚨 *ЧЕРЕЗ 2 ЧАСА \\| %s*\n", escMD2(label))
	msg += fmt.Sprintf("%s запускается в %s UTC\n",
		escMD2(e.Title), escMD2(e.Date.UTC().Format("15:04")))
	msg += fmt.Sprintf("💡 Стратегия: %s\n", escMD2(strategy))
	if e.URL != "" {
		msg += fmt.Sprintf("🔗 [Анонс](%s)\n", e.URL)
	}
	return msg
}

func eventMeta(e model.Event) (icon, label, strategy string) {
	switch e.Type {
	case model.EventLaunchpool:
		return "⏰", "LAUNCHPOOL", "Launchpool Harvest (3x, шорт за 12–24ч до листинга)"
	case model.EventListing:
		return "⏰", "ЛИСТИНГ", "Token Splash Short (4x, ждать RSI>85 + Volume>300%)"
	case model.EventUnlock:
		return "⏰", "РАЗЛОК", "VC-Gravity (шорт при разлоке >5% supply)"
	case model.EventAirdrop:
		return "⏰", "AIRDROP/TGE", "TGE Short (4x, продать на спайке первых минут)"
	}
	return "⏰", string(e.Type), ""
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
	return `🤖 *Crypto Calendar Bot*

Доступные команды:
/today — события сегодня
/tomorrow — события завтра
/week — события на неделю
/listings — предстоящие листинги
/unlocks — предстоящие разлоки
/airdrops — предстоящие аирдропы
/launchpools — предстоящие лаунчпулы
/digest — дайджест недели
/refresh — обновить данные сейчас`
}

// FormatEventList formats a list of events with a header for command responses.
func FormatEventList(events []model.Event, header string) string {
	if len(events) == 0 {
		return escMD2(header) + "\n\nСобытий не найдено\\."
	}

	var sb strings.Builder
	sb.WriteString("*" + escMD2(header) + "*\n")

	for _, e := range events {
		icon := eventIcon(e.Type)
		sb.WriteString(fmt.Sprintf("\n%s *%s* — %s\n",
			icon, escMD2(e.Token), escMD2(e.Title)))
		sb.WriteString(fmt.Sprintf("   📅 %s UTC\n",
			escMD2(e.Date.UTC().Format("2 Jan, 15:04"))))
		if e.Details != "" {
			sb.WriteString(fmt.Sprintf("   ℹ️ %s\n", escMD2(e.Details)))
		}
		if e.URL != "" {
			sb.WriteString(fmt.Sprintf("   🔗 [Анонс](%s)\n", e.URL))
		}
	}
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
