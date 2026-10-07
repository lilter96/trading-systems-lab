package news

import (
	"context"
	"io"
	"log/slog"
	"testing"
	"time"
)

func TestSyndicatedStory_ClusteredNotCountedFiveTimes(t *testing.T) {
	c := NewCleaner(map[string][]string{
		"BTC-USDT": {"BTC", "Bitcoin"},
	}, 30*time.Minute)

	articles := []RawArticle{
		{SourceID: "a:1", Tier: SourceTierTier2Established, URL: "https://example.com/1",
			RawText: "Bitcoin Surges Past $100K as Institutional Adoption Accelerates\n\nThe price of Bitcoin has exceeded $100,000 for the first time..."},
		{SourceID: "a:2", Tier: SourceTierTier2Established, URL: "https://example.com/2",
			RawText: "Bitcoin Surges Past $100K as Institutional Adoption Accelerates\n\nBTC reaches new all-time high amid growing institutional interest..."},
		{SourceID: "a:3", Tier: SourceTierTier2Established, URL: "https://example.com/3",
			RawText: "Bitcoin Surges Past $100K as Institutional Adoption Accelerates\n\nAnother syndicated copy of the same story..."},
	}

	cleaned := c.Clean(articles)
	if len(cleaned) != 1 {
		t.Errorf("expected 1 dedup cluster for 3 identical headlines, got %d", len(cleaned))
	}

	if len(cleaned) > 0 {
		if len(cleaned[0].TaggedInstruments) == 0 {
			t.Error("expected Bitcoin article to be tagged to BTC-USDT")
		}
	}
}

func TestNonEnglishArticle_ExcludedFromScoring(t *testing.T) {
	c := NewCleaner(nil, 10*time.Minute)

	articles := []RawArticle{
		{
			SourceID: "en:1", Tier: SourceTierTier2Established,
			RawText: "Bitcoin Price Analysis: Market Shows Strong Momentum",
		},
		{
			SourceID: "zh:1", Tier: SourceTierTier2Established,
			RawText: "比特币价格突破新高，市场情绪乐观",
		},
		{
			SourceID: "ru:1", Tier: SourceTierTier2Established,
			RawText: "Биткоин превысил $100,000 на фоне институционального принятия",
		},
	}

	cleaned := c.Clean(articles)
	if len(cleaned) != 1 {
		t.Errorf("expected 1 English article to pass, got %d (Chinese and Russian should be excluded)", len(cleaned))
	}
	if len(cleaned) > 0 && cleaned[0].Source.SourceID != "en:1" {
		t.Errorf("expected the English article to be the only one kept, got %q", cleaned[0].Source.SourceID)
	}

	total, excluded := c.Stats()
	if total != 3 {
		t.Errorf("expected 3 total articles processed, got %d", total)
	}
	if excluded != 2 {
		t.Errorf("expected 2 non-English excluded, got %d", excluded)
	}
}

func TestIrrelevantArticle_NotTaggedToInstrument(t *testing.T) {
	c := NewCleaner(map[string][]string{
		"BTC-USDT": {"BTC", "Bitcoin", "bitcoin"},
		"ETH-USDT": {"ETH", "Ethereum"},
	}, 10*time.Minute)

	articles := []RawArticle{
		{
			SourceID: "b:1", Tier: SourceTierTier2Established,
			RawText: "Apple Reports Record Q4 Earnings\n\nApple Inc. has reported its highest quarterly earnings ever...",
		},
	}

	cleaned := c.Clean(articles)
	if len(cleaned) != 1 {
		t.Fatalf("expected 1 article, got %d", len(cleaned))
	}
	if len(cleaned[0].TaggedInstruments) != 0 {
		t.Errorf("expected no instruments tagged for AAPL article, got %v", cleaned[0].TaggedInstruments)
	}
}

func TestSourceTier_PropagatesToCleanedArticle(t *testing.T) {
	c := NewCleaner(nil, 10*time.Minute)

	articles := []RawArticle{
		{
			SourceID: "t1:1", Tier: SourceTierTier1Wire,
			RawText: "Exchange Announces New Listing",
		},
		{
			SourceID: "t2:1", Tier: SourceTierTier2Established,
			RawText: "Crypto Market Update: Prices Rise",
		},
	}

	cleaned := c.Clean(articles)
	if len(cleaned) != 2 {
		t.Fatalf("expected 2 cleaned articles, got %d", len(cleaned))
	}

	tierMap := make(map[SourceTier]bool)
	for _, ca := range cleaned {
		tierMap[ca.Source.Tier] = true
	}

	if !tierMap[SourceTierTier1Wire] {
		t.Error("expected Tier 1 article to propagate its tier")
	}
	if !tierMap[SourceTierTier2Established] {
		t.Error("expected Tier 2 article to propagate its tier")
	}
}

func TestDedupWindow_ExpiredArticleGetsNewCluster(t *testing.T) {
	c := NewCleaner(nil, 1*time.Nanosecond)

	articles1 := []RawArticle{
		{SourceID: "a:1", RawText: "Same Headline Across Two Minutes"},
	}

	first := c.Clean(articles1)
	if len(first) != 1 {
		t.Fatal("expected first article to pass dedup")
	}

	time.Sleep(1 * time.Millisecond)

	articles2 := []RawArticle{
		{SourceID: "a:2", RawText: "Same Headline Across Two Minutes"},
	}

	second := c.Clean(articles2)
	if len(second) != 1 {
		t.Errorf("expected expired dedup window to allow the same headline, got %d articles", len(second))
	}

	if len(second) > 0 && second[0].DedupClusterID == "" {
		t.Error("expected a new cluster ID for the expired article")
	}
}

func TestEmptyFeed_ContinuesWithOtherFeeds(t *testing.T) {
	working := &mockFetcher{
		source: "working",
		tier:   SourceTierTier2Established,
		articles: []RawArticle{
			{SourceID: "w:1", RawText: "Crypto Market Update"},
		},
	}
	broken := &mockFetcher{
		source:  "broken",
		tier:    SourceTierTier2Established,
		err:     assertError("connection refused"),
	}

	mf := NewMultiFetcher(slog.New(slog.NewTextHandler(io.Discard, nil)), broken, working)
	articles, err := mf.Fetch(context.Background())
	if err != nil {
		t.Fatalf("expected partial success, got error: %v", err)
	}
	if len(articles) != 1 {
		t.Errorf("expected 1 article from working fetcher, got %d", len(articles))
	}
}

func TestCleaner_TagInstruments(t *testing.T) {
	c := NewCleaner(map[string][]string{
		"BTC-USDT": {"BTC", "Bitcoin", "bitcoin"},
		"ETH-USDT": {"ETH", "Ethereum", "ethereum"},
		"SOL-USDT": {"SOL", "Solana"},
	}, 10*time.Minute)

	tests := []struct {
		text     string
		expected []string
	}{
		{"Bitcoin surges to new highs", []string{"BTC-USDT"}},
		{"Ethereum and Solana lead the rally", []string{"ETH-USDT", "SOL-USDT"}},
		{"BTC and ETH both gaining", []string{"BTC-USDT", "ETH-USDT"}},
		{"No crypto mentions here", nil},
	}

	for _, tt := range tests {
		t.Run(tt.text[:20], func(t *testing.T) {
			got := c.tagInstruments(tt.text)
			if !stringSetsMatch(got, tt.expected) {
				t.Errorf("expected %v, got %v", tt.expected, got)
			}
		})
	}
}

func stringSetsMatch(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	m := make(map[string]int, len(a))
	for _, s := range a {
		m[s]++
	}
	for _, s := range b {
		m[s]--
		if m[s] < 0 {
			return false
		}
	}
	return true
}

func TestStripHTML(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"<p>Hello World</p>", "Hello World"},
		{"<a href=\"https://example.com\">Click here</a>", "Click here"},
		{"No HTML", "No HTML"},
		{"<div><span>Nested</span></div>", "Nested"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := stripHTML(tt.input)
			if got != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, got)
			}
		})
	}
}

func TestNormalizeText(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"Hello, World!", "Hello World"},
		{"Bitcoin's price: $100,000!", "Bitcoin s price 100 000"},
		{"  Extra   spaces  ", "Extra spaces"},
	}

	for _, tt := range tests {
		t.Run(tt.input[:10], func(t *testing.T) {
			got := normalizeText(tt.input)
			if got != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, got)
			}
		})
	}
}

type mockFetcher struct {
	source   string
	tier     SourceTier
	articles []RawArticle
	err      error
}

func (m *mockFetcher) Fetch(_ context.Context) ([]RawArticle, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.articles, nil
}

func (m *mockFetcher) Source() string    { return m.source }
func (m *mockFetcher) Tier() SourceTier  { return m.tier }

type assertError string

func (e assertError) Error() string { return string(e) }
