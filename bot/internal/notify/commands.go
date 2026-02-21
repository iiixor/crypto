package notify

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"crypto-bot/internal/calendar"
	"crypto-bot/internal/model"
)

// CommandHandler routes Telegram commands to the appropriate handlers.
type CommandHandler struct {
	tg  *Telegram
	agg *calendar.Aggregator
}

// NewCommandHandler creates a new CommandHandler.
func NewCommandHandler(tg *Telegram, agg *calendar.Aggregator) *CommandHandler {
	return &CommandHandler{tg: tg, agg: agg}
}

// Handle parses the command and dispatches to the right handler.
func (h *CommandHandler) Handle(chatID int64, text string) {
	// Strip @BotName suffix (sent in group chats: /cmd@BotName)
	cmd := strings.ToLower(strings.SplitN(text, " ", 2)[0])
	if at := strings.Index(cmd, "@"); at != -1 {
		cmd = cmd[:at]
	}

	switch cmd {
	case "/start":
		h.handleStart(chatID)
	case "/digest":
		h.handleDigest(chatID)
	case "/today":
		h.handleToday(chatID)
	case "/tomorrow":
		h.handleTomorrow(chatID)
	case "/week":
		h.handleWeek(chatID)
	case "/listings":
		h.handleByType(chatID, model.EventListing, "Предстоящие листинги")
	case "/unlocks":
		h.handleByType(chatID, model.EventUnlock, "Предстоящие разлоки")
	case "/airdrops":
		h.handleByType(chatID, model.EventAirdrop, "Предстоящие аирдропы / TGE")
	case "/launchpools":
		h.handleByType(chatID, model.EventLaunchpool, "Предстоящие лаунчпулы")
	case "/refresh":
		h.handleRefresh(chatID)
	}
}

func (h *CommandHandler) send(chatID int64, text string) {
	if err := h.tg.SendToChat(chatID, text); err != nil {
		log.Printf("[commands] send to %d failed: %v", chatID, err)
	}
}

func (h *CommandHandler) handleStart(chatID int64) {
	h.send(chatID, FormatHelp())
}

func (h *CommandHandler) handleDigest(chatID int64) {
	events := calendar.EventsForWeek(h.agg.Events())
	now := time.Now().UTC()
	weekEnd := now.Add(7 * 24 * time.Hour)
	// Reuse FormatDigest for the full week digest view
	msg := FormatDigest(events, now, weekEnd)
	h.send(chatID, msg)
}

func (h *CommandHandler) handleToday(chatID int64) {
	events := calendar.EventsToday(h.agg.Events())
	h.send(chatID, FormatEventList(events, "События сегодня"))
}

func (h *CommandHandler) handleTomorrow(chatID int64) {
	events := calendar.EventsTomorrowAll(h.agg.Events())
	h.send(chatID, FormatEventList(events, "События завтра"))
}

func (h *CommandHandler) handleWeek(chatID int64) {
	events := calendar.EventsForWeek(h.agg.Events())
	h.send(chatID, FormatEventList(events, "События на неделю"))
}

func (h *CommandHandler) handleByType(chatID int64, evType model.EventType, header string) {
	events := calendar.EventsUpcoming(h.agg.Events(), evType)
	h.send(chatID, FormatEventList(events, header))
}

func (h *CommandHandler) handleRefresh(chatID int64) {
	if err := h.tg.SendToChat(chatID, "🔄 Обновляю\\.\\.\\."); err != nil {
		log.Printf("[commands] refresh ack send failed: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	events := h.agg.Refresh(ctx)
	msg := fmt.Sprintf("✅ Обновлено: найдено *%d* событий", len(events))
	h.send(chatID, msg)
}
