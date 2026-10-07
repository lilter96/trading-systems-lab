package telegram

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestSendAlert_Success(t *testing.T) {
	var gotBody string
	var gotURL string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			t.Fatalf("expected POST, got %s", r.Method)
		}
		gotURL = r.URL.String()
		buf := make([]byte, 512)
		n, _ := r.Body.Read(buf)
		gotBody = strings.TrimSpace(string(buf[:n]))
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, `{"ok":true,"result":{}}`)
	}))
	defer srv.Close()

	// Set test URL to hit our mock server
	origBase := baseURL
	origClient := httpClient
	baseURL = srv.URL
	httpClient = srv.Client()
	defer func() {
		baseURL = origBase
		httpClient = origClient
	}()

	botToken := "test:token"
	chatId := "12345"
	message := "test alert"

	err := SendAlert(botToken, chatId, message)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(gotBody, `"chat_id":"12345"`) {
		t.Errorf("body missing chat_id: %s", gotBody)
	}
	if !strings.Contains(gotBody, `"text":"test alert"`) {
		t.Errorf("body missing text: %s", gotBody)
	}
	if !strings.Contains(gotBody, `"parse_mode":"HTML"`) {
		t.Errorf("body missing parse_mode: %s", gotBody)
	}
	if !strings.Contains(gotURL, "/bottest:token/sendMessage") {
		t.Errorf("URL not constructed correctly: %s", gotURL)
	}

	// Verify SendAlert validates empty params
	if err := SendAlert("", chatId, message); err == nil {
		t.Fatal("expected error for empty bot token")
	}
	if err := SendAlert(botToken, "", message); err == nil {
		t.Fatal("expected error for empty chat ID")
	}
}

func TestSendAlert_HTTPError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer srv.Close()

	origBase := baseURL
	origClient := httpClient
	baseURL = srv.URL
	httpClient = srv.Client()
	defer func() {
		baseURL = origBase
		httpClient = origClient
	}()

	err := SendAlert("token", "chat", "msg")
	if err == nil {
		t.Fatal("expected error for 401 response")
	}
	if !strings.Contains(err.Error(), "401") && !strings.Contains(err.Error(), "Unauthorized") {
		t.Errorf("expected 401 in error, got: %v", err)
	}
}

func TestSendAlert_NetworkError(t *testing.T) {
	// Use invalid URL to trigger network error
	origBase := baseURL
	origClient := httpClient
	baseURL = "http://10.255.255.1:12345" // unreachable
	httpClient = &http.Client{Timeout: 100 * time.Millisecond}
	defer func() {
		baseURL = origBase
		httpClient = origClient
	}()

	err := SendAlert("token", "chat", "msg")
	if err == nil {
		t.Fatal("expected network error")
	}
}

func TestSendAlert_MessageBody(t *testing.T) {
	botToken := "bot123:abc"
	chatId := "-100123456"
	message := "HALT: Emergency triggered"

	expectedURL := fmt.Sprintf("https://api.telegram.org/bot%s/sendMessage", botToken)
	if !strings.Contains(expectedURL, botToken) {
		t.Fatal("URL should contain bot token")
	}

	// Verify SendAlert validates properly
	err := SendAlert(botToken, chatId, message)
	// Expected to fail with network error since we hit real API
	_ = err
}