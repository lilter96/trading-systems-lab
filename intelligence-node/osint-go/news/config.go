package news

import (
	"os"
	"strconv"
	"time"
)

type RSSSource struct {
	Name string
	URL  string
	Tier SourceTier
}

type NewsConfig struct {
	PollInterval  time.Duration
	StaleThreshold int
	Sources       []RSSSource
	TickerAliases map[string][]string
}

func DefaultConfig() *NewsConfig {
	return &NewsConfig{
		PollInterval:   10 * time.Minute,
		StaleThreshold: 3,
		Sources: []RSSSource{
			{Name: "Kraken Blog", URL: "https://blog.kraken.com/feed", Tier: SourceTierTier1Wire},
			{Name: "CoinDesk", URL: "https://www.coindesk.com/arc/outboundfeeds/rss/", Tier: SourceTierTier2Established},
			{Name: "CoinTelegraph", URL: "https://cointelegraph.com/rss", Tier: SourceTierTier2Established},
			{Name: "Decrypt", URL: "https://decrypt.co/feed", Tier: SourceTierTier2Established},
			{Name: "The Block", URL: "https://www.theblock.co/rss.xml", Tier: SourceTierTier2Established},
			{Name: "Bitcoin Magazine", URL: "https://bitcoinmagazine.com/feed", Tier: SourceTierTier2Established},
			{Name: "CryptoSlate", URL: "https://cryptoslate.com/feed/", Tier: SourceTierTier2Established},
			{Name: "BeInCrypto", URL: "https://beincrypto.com/feed/", Tier: SourceTierTier2Established},
		},
		TickerAliases: map[string][]string{
			"BTC-USDT": {"BTC", "Bitcoin", "$BTC", "bitcoin"},
			"ETH-USDT": {"ETH", "Ethereum", "$ETH", "ethereum"},
			"SOL-USDT": {"SOL", "Solana", "$SOL", "solana"},
		},
	}
}

func LoadNewsConfigFromEnv() *NewsConfig {
	cfg := DefaultConfig()

	if v := os.Getenv("NEWS_POLL_INTERVAL_MINUTES"); v != "" {
		if min, err := strconv.Atoi(v); err == nil && min > 0 {
			cfg.PollInterval = time.Duration(min) * time.Minute
		}
	}

	if v := os.Getenv("NEWS_STALE_THRESHOLD"); v != "" {
		if t, err := strconv.Atoi(v); err == nil && t > 0 {
			cfg.StaleThreshold = t
		}
	}

	return cfg
}
