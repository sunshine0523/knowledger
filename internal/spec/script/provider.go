package script

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"

	"github.com/kindbrave/claude-knowledger/internal/config"
	"github.com/kindbrave/claude-knowledger/internal/core"
)

// Provider runs a custom script and parses its stdout into Findings.
type Provider struct {
	source config.SourceConfig
}

func New(source config.SourceConfig) *Provider {
	return &Provider{source: source}
}

func (p *Provider) Run(ctx context.Context, specID string, changedFiles []string) ([]core.Finding, []core.RuleSet, error) {
	if p.source.Script == "" {
		return nil, nil, fmt.Errorf("spec %s: script path is empty", specID)
	}

	cmd := exec.CommandContext(ctx, p.source.Script)
	cmd.Env = append(cmd.Environ(), "CHANGED_FILES="+strings.Join(changedFiles, "\n"))

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	_ = cmd.Run()

	format := p.source.OutputFormat
	if format == "" {
		format = "json"
	}

	switch format {
	case "json":
		var findings []core.Finding
		if stdout.Len() == 0 {
			return nil, nil, nil
		}
		if err := json.Unmarshal(stdout.Bytes(), &findings); err != nil {
			return nil, nil, fmt.Errorf("spec %s: json parse error: %w", specID, err)
		}
		for i := range findings {
			if findings[i].SpecID == "" {
				findings[i].SpecID = specID
			}
		}
		return findings, nil, nil
	default: // text
		msg := strings.TrimSpace(stdout.String())
		if msg == "" {
			return nil, nil, nil
		}
		return []core.Finding{{
			SpecID:   specID,
			Severity: "should-fix",
			Message:  msg,
		}}, nil, nil
	}
}
