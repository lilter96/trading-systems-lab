package telegram

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

type sendMessageReq struct {
	ChatId    string `json:"chat_id"`
	Text      string `json:"text"`
	ParseMode string `json:"parse_mode,omitempty"`
}

var (
	baseURL    = "https://api.telegram.org"
	httpClient = http.DefaultClient
)

func SetTestBaseURL(url string, client *http.Client) {
	baseURL = url
	httpClient = client
}

func SendAlert(botToken, chatId, message string) error {
	if botToken == "" || chatId == "" {
		return fmt.Errorf("telegram bot token or chat ID is empty")
	}

	body := sendMessageReq{
		ChatId:    chatId,
		Text:      message,
		ParseMode: "HTML",
	}
	payload, err := json.Marshal(body)
	if err != nil {
		return fmt.Errorf("marshal: %w", err)
	}

	url := fmt.Sprintf("%s/bot%s/sendMessage", baseURL, botToken)

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(payload))
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("http post: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("telegram API returned status %d", resp.StatusCode)
	}
	return nil
}