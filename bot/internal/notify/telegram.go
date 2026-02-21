package notify

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// Telegram отправляет сообщения через Bot API
type Telegram struct {
	token      string
	chatID     string
	client     *http.Client
	pollClient *http.Client // longer timeout for long-polling getUpdates
}

func NewTelegram(token, chatID string) *Telegram {
	return &Telegram{
		token:      token,
		chatID:     chatID,
		client:     &http.Client{Timeout: 10 * time.Second},
		pollClient: &http.Client{Timeout: 40 * time.Second},
	}
}

// Structs for receiving updates via getUpdates
type tgUpdate struct {
	UpdateID int64      `json:"update_id"`
	Message  *tgMessage `json:"message"`
}

type tgMessage struct {
	MessageID int64   `json:"message_id"`
	Chat      tgChat  `json:"chat"`
	Text      string  `json:"text"`
}

type tgChat struct {
	ID int64 `json:"id"`
}

type tgUpdatesResponse struct {
	OK          bool       `json:"ok"`
	Result      []tgUpdate `json:"result"`
	ErrorCode   int        `json:"error_code"`
	Description string     `json:"description"`
}

// DeleteWebhook removes any active webhook so that getUpdates works.
// Must be called before starting long-polling. Safe to call when no webhook is set.
func (t *Telegram) DeleteWebhook() error {
	url := fmt.Sprintf("https://api.telegram.org/bot%s/deleteWebhook", t.token)
	resp, err := t.client.Get(url)
	if err != nil {
		return fmt.Errorf("deleteWebhook: %w", err)
	}
	defer resp.Body.Close()

	var tgResp tgResponse
	if err := json.NewDecoder(resp.Body).Decode(&tgResp); err != nil {
		return fmt.Errorf("decode: %w", err)
	}
	if !tgResp.OK {
		return fmt.Errorf("telegram error: %s", tgResp.Description)
	}
	return nil
}

// GetUpdates polls Telegram for new updates using long-polling.
// Returns new updates and the next offset to use.
func (t *Telegram) GetUpdates(offset int64, timeout int) ([]tgUpdate, int64, error) {
	url := fmt.Sprintf("https://api.telegram.org/bot%s/getUpdates", t.token)

	// Use POST with JSON body to pass allowed_updates cleanly
	body, _ := json.Marshal(map[string]interface{}{
		"offset":          offset,
		"timeout":         timeout,
		"allowed_updates": []string{"message"},
	})

	resp, err := t.pollClient.Post(url, "application/json", bytes.NewReader(body))
	if err != nil {
		return nil, offset, fmt.Errorf("getUpdates: %w", err)
	}
	defer resp.Body.Close()

	var upResp tgUpdatesResponse
	if err := json.NewDecoder(resp.Body).Decode(&upResp); err != nil {
		return nil, offset, fmt.Errorf("decode updates: %w", err)
	}
	if !upResp.OK {
		return nil, offset, fmt.Errorf("telegram getUpdates error %d: %s", upResp.ErrorCode, upResp.Description)
	}

	var nextOffset int64 = offset
	for _, u := range upResp.Result {
		if u.UpdateID+1 > nextOffset {
			nextOffset = u.UpdateID + 1
		}
	}
	return upResp.Result, nextOffset, nil
}

// SendToChat sends a MarkdownV2 message to a specific chat ID (for command responses).
func (t *Telegram) SendToChat(chatID int64, text string) error {
	url := fmt.Sprintf("https://api.telegram.org/bot%s/sendMessage", t.token)

	body, err := json.Marshal(map[string]interface{}{
		"chat_id":                  chatID,
		"text":                     text,
		"parse_mode":               "MarkdownV2",
		"disable_web_page_preview": true,
	})
	if err != nil {
		return fmt.Errorf("marshal: %w", err)
	}

	resp, err := t.client.Post(url, "application/json", bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("http post: %w", err)
	}
	defer resp.Body.Close()

	var tgResp tgResponse
	if err := json.NewDecoder(resp.Body).Decode(&tgResp); err != nil {
		return fmt.Errorf("decode response: %w", err)
	}
	if !tgResp.OK {
		return fmt.Errorf("telegram error: %s", tgResp.Description)
	}
	return nil
}

type tgRequest struct {
	ChatID    string `json:"chat_id"`
	Text      string `json:"text"`
	ParseMode string `json:"parse_mode"`
	// Отключаем предпросмотр ссылок чтобы не шумело
	DisableWebPagePreview bool `json:"disable_web_page_preview"`
}

type tgResponse struct {
	OK          bool   `json:"ok"`
	Description string `json:"description"`
}

// Send отправляет сообщение в Telegram (Markdown V2)
func (t *Telegram) Send(text string) error {
	url := fmt.Sprintf("https://api.telegram.org/bot%s/sendMessage", t.token)

	body, err := json.Marshal(tgRequest{
		ChatID:                t.chatID,
		Text:                  text,
		ParseMode:             "MarkdownV2",
		DisableWebPagePreview: true,
	})
	if err != nil {
		return fmt.Errorf("marshal: %w", err)
	}

	resp, err := t.client.Post(url, "application/json", bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("http post: %w", err)
	}
	defer resp.Body.Close()

	var tgResp tgResponse
	if err := json.NewDecoder(resp.Body).Decode(&tgResp); err != nil {
		return fmt.Errorf("decode response: %w", err)
	}
	if !tgResp.OK {
		return fmt.Errorf("telegram error: %s", tgResp.Description)
	}
	return nil
}

// SendPlain отправляет без разметки (для отладки)
func (t *Telegram) SendPlain(text string) error {
	url := fmt.Sprintf("https://api.telegram.org/bot%s/sendMessage", t.token)

	body, err := json.Marshal(map[string]interface{}{
		"chat_id":                  t.chatID,
		"text":                     text,
		"disable_web_page_preview": true,
	})
	if err != nil {
		return fmt.Errorf("marshal: %w", err)
	}

	resp, err := t.client.Post(url, "application/json", bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("http post: %w", err)
	}
	defer resp.Body.Close()

	var tgResp tgResponse
	if err := json.NewDecoder(resp.Body).Decode(&tgResp); err != nil {
		return fmt.Errorf("decode response: %w", err)
	}
	if !tgResp.OK {
		return fmt.Errorf("telegram error: %s", tgResp.Description)
	}
	return nil
}
