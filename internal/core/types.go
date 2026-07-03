package core

import "time"

type KnowledgeBase struct {
	ID                string
	Scope             string
	Name              string
	StoreType         string
	StoreConfig       map[string]any
	Enabled           bool
	DefaultSearchMode string
	Indexing          map[string]any
	Tags              []string
}

type KnowledgeItem struct {
	ID        string
	KBID      string
	Type      string
	Title     string
	Content   string
	Summary   string
	SourceRef string
	Metadata  map[string]any
	Tags      []string
	CreatedAt time.Time
	UpdatedAt time.Time
}

type SearchHit struct {
	ItemID         string
	KBID           string
	Scope          string
	ItemType       string
	Title          string
	Snippet        string
	ContentPreview string
	Score          float64
	MatchMode      string
	SourceBackend  string
	Locator        string
	Metadata       map[string]any
}

type IngestionResult struct {
	Success     bool
	ItemID      string
	IndexQueued bool
	Warnings    []string
}

type IndexStatus struct {
	State         string
	LastSuccessAt *time.Time
	LastError     string
}

type Finding struct {
	SpecID       string `json:"spec_id"`
	RuleID       string `json:"rule_id,omitempty"`
	Path         string `json:"path"`
	Line         int    `json:"line,omitempty"`
	Severity     string `json:"severity"`           // must-fix | should-fix | nit
	Message      string `json:"message"`
	SuggestedFix string `json:"suggested_fix,omitempty"`
	RuleQuote    string `json:"rule_quote,omitempty"`
}

type RuleSet struct {
	SpecID string          `json:"spec_id"`
	Items  []KnowledgeItem `json:"items"`
}

type LintResult struct {
	Findings []Finding `json:"findings"`
	RuleSets []RuleSet `json:"rule_sets"`
	Errors   []string  `json:"errors,omitempty"`
}
