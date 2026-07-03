package spec

import (
	"context"

	"github.com/kindbrave/claude-knowledger/internal/core"
)

// Provider executes a single specification and returns its results.
type Provider interface {
	Run(ctx context.Context, specID string, changedFiles []string) ([]core.Finding, []core.RuleSet, error)
}
