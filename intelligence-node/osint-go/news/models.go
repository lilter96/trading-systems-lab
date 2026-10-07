package news

import "time"

type SourceTier int

const (
	SourceTierTier1Wire        SourceTier = 1
	SourceTierTier2Established SourceTier = 2
	SourceTierTier3Social      SourceTier = 3
)

func (t SourceTier) String() string {
	switch t {
	case SourceTierTier1Wire:
		return "tier1_wire"
	case SourceTierTier2Established:
		return "tier2_established"
	case SourceTierTier3Social:
		return "tier3_social"
	default:
		return "unknown"
	}
}

type RawArticle struct {
	SourceID         string
	Tier             SourceTier
	URL              string
	PublishedAtUTC   time.Time
	RetrievedAtUTC   time.Time
	RawText          string
	DetectedLanguage string
}

type CleanedArticle struct {
	Source            RawArticle
	CleanedText       string
	DedupClusterID    string
	TaggedInstruments []string
}

type NewsSnapshot struct {
	Articles int
	Stale    bool
	Sources  map[string]SourceHealth
}

type SourceHealth struct {
	LastSuccess time.Time
	Failures    int
	IsStale     bool
}
