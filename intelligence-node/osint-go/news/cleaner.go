package news

import (
	"crypto/sha256"
	"fmt"
	"regexp"
	"strings"
	"sync"
	"time"
	"unicode"
)

type Cleaner struct {
	aliases  map[string][]string
	patterns map[string]*regexp.Regexp
	seen     map[string]time.Time
	mu       sync.Mutex
	window   time.Duration

	totalArticles  int
	excludedLang   int
}

func NewCleaner(aliases map[string][]string, dedupWindow time.Duration) *Cleaner {
	patterns := make(map[string]*regexp.Regexp, len(aliases))
	for instrument, words := range aliases {
		escaped := make([]string, len(words))
		for i, w := range words {
			escaped[i] = regexp.QuoteMeta(w)
		}
		pattern := fmt.Sprintf(`(?i)\b(%s)\b`, strings.Join(escaped, "|"))
		patterns[instrument] = regexp.MustCompile(pattern)
	}

	return &Cleaner{
		aliases:  aliases,
		patterns: patterns,
		seen:     make(map[string]time.Time),
		window:   dedupWindow,
	}
}

func (c *Cleaner) Clean(articles []RawArticle) []CleanedArticle {
	if len(articles) == 0 {
		return nil
	}

	result := make([]CleanedArticle, 0, len(articles))

	for _, raw := range articles {
		c.totalArticles++

		if !isLatinScript(raw.RawText) {
			c.excludedLang++
			continue
		}

		headline := extractHeadline(raw.RawText)
		clusterID := c.dedup(headline)
		if clusterID == "" {
			continue
		}

		plainText := normalizeText(raw.RawText)
		instruments := c.tagInstruments(plainText + " " + headline)

		result = append(result, CleanedArticle{
			Source:            raw,
			CleanedText:       plainText,
			DedupClusterID:    clusterID,
			TaggedInstruments: instruments,
		})
	}

	return result
}

func isLatinScript(text string) bool {
	if strings.TrimSpace(text) == "" {
		return false
	}

	const threshold = 0.2
	nonLatin := 0
	total := 0

	for _, r := range text {
		if r == '\n' || r == '\t' || r == ' ' {
			continue
		}
		total++
		if r > 0x02AF {
			nonLatin++
		}
	}

	if total == 0 {
		return false
	}

	return float64(nonLatin)/float64(total) <= threshold
}

func extractHeadline(rawText string) string {
	parts := strings.SplitN(rawText, "\n\n", 2)
	if len(parts) == 0 {
		return ""
	}
	return strings.TrimSpace(parts[0])
}

func normalizeText(s string) string {
	s = strings.TrimSpace(s)
	var b strings.Builder
	for _, r := range s {
		if unicode.IsPunct(r) || unicode.IsSymbol(r) {
			b.WriteRune(' ')
		} else {
			b.WriteRune(r)
		}
	}
	return strings.Join(strings.Fields(b.String()), " ")
}

func (c *Cleaner) dedup(headline string) string {
	normalized := strings.ToLower(strings.TrimSpace(headline))
	normalized = strings.Join(strings.Fields(normalized), " ")

	hash := fmt.Sprintf("%x", sha256.Sum256([]byte(normalized)))

	c.mu.Lock()
	defer c.mu.Unlock()

	if seenAt, ok := c.seen[hash]; ok {
		if time.Since(seenAt) <= c.window {
			return ""
		}
	}

	c.seen[hash] = time.Now()
	return hash
}

func (c *Cleaner) tagInstruments(text string) []string {
	var found []string
	seen := make(map[string]bool)

	for instrument, pattern := range c.patterns {
		if pattern.MatchString(text) {
			if !seen[instrument] {
				found = append(found, instrument)
				seen[instrument] = true
			}
		}
	}

	return found
}

func (c *Cleaner) Stats() (total, excluded int) {
	return c.totalArticles, c.excludedLang
}
