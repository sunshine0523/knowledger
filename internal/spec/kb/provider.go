package kb

import (
	"context"

	"github.com/kindbrave/claude-knowledger/internal/config"
	"github.com/kindbrave/claude-knowledger/internal/core"
	"github.com/kindbrave/claude-knowledger/internal/service"
)

// Provider fetches knowledge items from a KB and returns them as a RuleSet
// for the main agent LLM to evaluate against the diff.
type Provider struct {
	svc    *service.Service
	source config.SourceConfig
	scope  string
}

func New(svc *service.Service, source config.SourceConfig, scope string) *Provider {
	return &Provider{svc: svc, source: source, scope: scope}
}

func (p *Provider) Run(ctx context.Context, specID string, _ []string) ([]core.Finding, []core.RuleSet, error) {
	items, err := p.svc.ListKnowledgeItems(ctx, p.scope, p.source.KBID)
	if err != nil {
		return nil, nil, err
	}
	if len(p.source.Tags) > 0 {
		items = filterByTags(items, p.source.Tags)
	}
	if len(items) == 0 {
		return nil, nil, nil
	}
	return nil, []core.RuleSet{{SpecID: specID, Items: items}}, nil
}

func filterByTags(items []core.KnowledgeItem, tags []string) []core.KnowledgeItem {
	tagSet := make(map[string]bool, len(tags))
	for _, t := range tags {
		tagSet[t] = true
	}
	var out []core.KnowledgeItem
	for _, item := range items {
		for _, t := range item.Tags {
			if tagSet[t] {
				out = append(out, item)
				break
			}
		}
	}
	return out
}
