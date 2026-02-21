package model

import "time"

// EventType — тип крипто-события
type EventType string

const (
	EventLaunchpool EventType = "launchpool"
	EventListing    EventType = "listing"
	EventUnlock     EventType = "unlock"
	EventAirdrop    EventType = "airdrop"
)

// Event — одно крипто-событие
type Event struct {
	ID       string    `json:"id"`       // уникальный идентификатор (source:token:date)
	Type     EventType `json:"type"`     // launchpool | listing | unlock | airdrop
	Source   string    `json:"source"`   // binance | bybit | okx | tokenunlocks | airdrops
	Token    string    `json:"token"`    // тикер токена, напр. VANA
	Title    string    `json:"title"`    // полное название события
	Date     time.Time `json:"date"`     // дата/время события (UTC)
	URL      string    `json:"url"`      // ссылка на анонс
	Details  string    `json:"details"`  // доп. данные (пары, % разлока и т.д.)

	// Флаги отправки — чтобы не дублировать уведомления
	SentDigest bool `json:"sent_digest"`
	Sent24h    bool `json:"sent_24h"`
	Sent2h     bool `json:"sent_2h"`
}
