package spec

import (
	"context"
	"fmt"
	"sync"

	"github.com/kindbrave/claude-knowledger/internal/config"
	"github.com/kindbrave/claude-knowledger/internal/core"
)

// Engine routes each enabled specification to its provider and aggregates results.
type Engine struct {
	specs     []config.SpecificationConfig
	providers map[string]Provider // keyed by spec id
}

// NewEngine builds an Engine from the loaded config. The caller must supply
// a provider factory via WithProvider; unknown spec types are silently skipped.
func NewEngine(specs []config.SpecificationConfig) *Engine {
	return &Engine{specs: specs, providers: make(map[string]Provider)}
}

func (e *Engine) RegisterProvider(specID string, p Provider) {
	e.providers[specID] = p
}

// Run executes all enabled specifications in parallel and returns a LintResult.
func (e *Engine) Run(ctx context.Context, changedFiles []string, specIDs []string) core.LintResult {
	filter := toSet(specIDs)
	type work struct {
		id string
		p  Provider
	}
	var tasks []work
	for _, s := range e.specs {
		if !s.Enabled {
			continue
		}
		if len(filter) > 0 && !filter[s.ID] {
			continue
		}
		p, ok := e.providers[s.ID]
		if !ok {
			continue
		}
		tasks = append(tasks, work{id: s.ID, p: p})
	}

	type result struct {
		findings []core.Finding
		ruleSets []core.RuleSet
		err      error
	}
	results := make([]result, len(tasks))
	var wg sync.WaitGroup
	for i, t := range tasks {
		wg.Add(1)
		go func(idx int, w work) {
			defer wg.Done()
			findings, ruleSets, err := w.p.Run(ctx, w.id, changedFiles)
			results[idx] = result{findings: findings, ruleSets: ruleSets, err: err}
		}(i, t)
	}
	wg.Wait()

	var out core.LintResult
	for i, r := range results {
		if r.err != nil {
			out.Errors = append(out.Errors, fmt.Sprintf("%s: %v", tasks[i].id, r.err))
			continue
		}
		out.Findings = append(out.Findings, r.findings...)
		out.RuleSets = append(out.RuleSets, r.ruleSets...)
	}
	return out
}

func toSet(ids []string) map[string]bool {
	if len(ids) == 0 {
		return nil
	}
	s := make(map[string]bool, len(ids))
	for _, id := range ids {
		s[id] = true
	}
	return s
}
