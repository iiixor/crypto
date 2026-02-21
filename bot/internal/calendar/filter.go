package calendar

import (
	"sort"
	"time"

	"crypto-bot/internal/model"
)

// EventsForWeek возвращает события за последние 14 дней и следующие 7 дней
func EventsForWeek(events []model.Event) []model.Event {
	from := time.Now().UTC().Add(-14 * 24 * time.Hour)
	end := time.Now().UTC().Add(7 * 24 * time.Hour)
	return filterAndSort(events, from, end)
}

// EventsTomorrow возвращает события завтра (для алерта за 24ч), которые ещё не отправлены
func EventsTomorrow(events []model.Event) []model.Event {
	now := time.Now().UTC()
	// Окно: 20–28 часов вперёд (чтобы не дублировать с более ранними проверками)
	from := now.Add(20 * time.Hour)
	to := now.Add(28 * time.Hour)

	var out []model.Event
	for _, e := range events {
		if !e.Sent24h && e.Date.After(from) && e.Date.Before(to) {
			out = append(out, e)
		}
	}
	return sortByDate(out)
}

// EventsIn2Hours возвращает события через ~2 часа (листинги и TGE), которые ещё не отправлены
func EventsIn2Hours(events []model.Event) []model.Event {
	now := time.Now().UTC()
	from := now.Add(90 * time.Minute)
	to := now.Add(150 * time.Minute)

	var out []model.Event
	for _, e := range events {
		// Алерт за 2ч только для листингов и TGE/airdrop
		if e.Type != model.EventListing && e.Type != model.EventAirdrop {
			continue
		}
		if !e.Sent2h && e.Date.After(from) && e.Date.Before(to) {
			out = append(out, e)
		}
	}
	return sortByDate(out)
}

// EventsForDigest возвращает события (последние 14 дней + 7 дней вперёд), не попавшие в предыдущий дайджест
func EventsForDigest(events []model.Event) []model.Event {
	from := time.Now().UTC().Add(-14 * 24 * time.Hour)
	end := time.Now().UTC().Add(7 * 24 * time.Hour)
	var out []model.Event
	for _, e := range filterAndSort(events, from, end) {
		if !e.SentDigest {
			out = append(out, e)
		}
	}
	return out
}

// EventsToday returns all events happening today (00:00 – 23:59 UTC).
func EventsToday(events []model.Event) []model.Event {
	now := time.Now().UTC()
	from := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
	to := from.Add(24 * time.Hour)
	return filterAndSort(events, from, to)
}

// EventsTomorrowAll returns all events tomorrow (ignores Sent24h — for on-demand query).
func EventsTomorrowAll(events []model.Event) []model.Event {
	now := time.Now().UTC()
	tomorrow := now.Add(24 * time.Hour)
	from := time.Date(tomorrow.Year(), tomorrow.Month(), tomorrow.Day(), 0, 0, 0, 0, time.UTC)
	to := from.Add(24 * time.Hour)
	return filterAndSort(events, from, to)
}

// EventsUpcoming returns all future events of the given type, up to 30 days ahead.
func EventsUpcoming(events []model.Event, evType model.EventType) []model.Event {
	now := time.Now().UTC()
	to := now.Add(30 * 24 * time.Hour)
	var out []model.Event
	for _, e := range events {
		if e.Type == evType && e.Date.After(now) && e.Date.Before(to) {
			out = append(out, e)
		}
	}
	return sortByDate(out)
}

func filterAndSort(events []model.Event, from, to time.Time) []model.Event {
	var out []model.Event
	for _, e := range events {
		if e.Date.After(from) && e.Date.Before(to) {
			out = append(out, e)
		}
	}
	return sortByDate(out)
}

func sortByDate(events []model.Event) []model.Event {
	sort.Slice(events, func(i, j int) bool {
		return events[i].Date.Before(events[j].Date)
	})
	return events
}
