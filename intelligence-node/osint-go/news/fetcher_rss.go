package news

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/mmcdole/gofeed"
)

type RSSFetcher struct {
	name   string
	url    string
	tier   SourceTier
	parser *gofeed.Parser
}

func NewRSSFetcher(name, url string, tier SourceTier) *RSSFetcher {
	return &RSSFetcher{
		name:   name,
		url:    url,
		tier:   tier,
		parser: gofeed.NewParser(),
	}
}

func (f *RSSFetcher) Source() string  { return f.name }
func (f *RSSFetcher) Tier() SourceTier { return f.tier }

func (f *RSSFetcher) Fetch(ctx context.Context) ([]RawArticle, error) {
	feed, err := f.parser.ParseURLWithContext(f.url, ctx)
	if err != nil {
		return nil, fmt.Errorf("rss fetch %q: %w", f.name, err)
	}

	now := time.Now().UTC()
	articles := make([]RawArticle, 0, len(feed.Items))

	for _, item := range feed.Items {
		published := parseFeedTime(item)
		if published.IsZero() {
			continue
		}

		rawText := buildRawText(item)

		sourceID := item.GUID
		if sourceID == "" {
			sourceID = item.Link
		}
		if sourceID == "" {
			sourceID = item.Title
		}

		articles = append(articles, RawArticle{
			SourceID:       f.name + ":" + sourceID,
			Tier:           f.tier,
			URL:            item.Link,
			PublishedAtUTC: published,
			RetrievedAtUTC: now,
			RawText:        rawText,
		})
	}

	if len(articles) == 0 {
		return nil, fmt.Errorf("rss %q: no items with valid dates", f.name)
	}

	return articles, nil
}

func parseFeedTime(item *gofeed.Item) time.Time {
	if item.PublishedParsed != nil {
		return item.PublishedParsed.UTC()
	}
	if item.UpdatedParsed != nil {
		return item.UpdatedParsed.UTC()
	}
	return time.Time{}
}

func buildRawText(item *gofeed.Item) string {
	var parts []string

	if item.Title != "" {
		parts = append(parts, strings.TrimSpace(item.Title))
	}

	if item.Description != "" {
		desc := strings.TrimSpace(item.Description)
		desc = stripHTML(desc)
		if desc != "" {
			parts = append(parts, desc)
		}
	}

	if item.Content != "" {
		content := strings.TrimSpace(item.Content)
		content = stripHTML(content)
		if content != "" {
			parts = append(parts, content)
		}
	}

	return strings.Join(parts, "\n\n")
}

func stripHTML(s string) string {
	var b strings.Builder
	inTag := false
	for _, r := range s {
		switch {
		case r == '<':
			inTag = true
		case r == '>':
			inTag = false
		case !inTag:
			b.WriteRune(r)
		}
	}
	return b.String()
}
