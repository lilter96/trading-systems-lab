package okxclient

import (
	"bufio"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

type Credentials struct {
	APIKey     string
	SecretKey  string
	Passphrase string
}

func LoadCredentialsFromEnv() (Credentials, error) {
	key := os.Getenv("OKX_API_KEY")
	secret := os.Getenv("OKX_SECRET_KEY")
	passphrase := os.Getenv("OKX_PASSPHRASE")

	if key == "" || secret == "" || passphrase == "" {
		return Credentials{}, fmt.Errorf("OKX_API_KEY, OKX_SECRET_KEY, and OKX_PASSPHRASE must be set")
	}

	return Credentials{
		APIKey:     key,
		SecretKey:  secret,
		Passphrase: passphrase,
	}, nil
}

func LoadCredentialsFromFile(path string) (Credentials, error) {
	f, err := os.Open(path)
	if err != nil {
		return Credentials{}, fmt.Errorf("open %s: %w", path, err)
	}
	defer f.Close()

	var creds Credentials
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}
		key := strings.TrimSpace(parts[0])
		val := strings.TrimSpace(parts[1])
		switch key {
		case "OKX_API_KEY":
			creds.APIKey = val
		case "OKX_SECRET_KEY":
			creds.SecretKey = val
		case "OKX_PASSPHRASE":
			creds.Passphrase = val
		}
	}
	if err := scanner.Err(); err != nil {
		return Credentials{}, fmt.Errorf("read %s: %w", path, err)
	}
	if creds.APIKey == "" || creds.SecretKey == "" || creds.Passphrase == "" {
		return Credentials{}, fmt.Errorf("missing one or more required env vars in %s", path)
	}
	return creds, nil
}

func (c *Credentials) Sign(method, requestPath, body string) (timestamp, signature string) {
	ts := time.Now().UTC().Format("2006-01-02T15:04:05.000Z")

	mac := hmac.New(sha256.New, []byte(c.SecretKey))
	mac.Write([]byte(ts + method + requestPath + body))
	sig := base64.StdEncoding.EncodeToString(mac.Sum(nil))

	return ts, sig
}

func (c *Credentials) WsLoginSign() (timestamp, signature string) {
	ts := strconv.FormatInt(time.Now().Unix(), 10)

	mac := hmac.New(sha256.New, []byte(c.SecretKey))
	mac.Write([]byte(ts + "GET" + "/users/self/verify"))
	sig := base64.StdEncoding.EncodeToString(mac.Sum(nil))

	return ts, sig
}
