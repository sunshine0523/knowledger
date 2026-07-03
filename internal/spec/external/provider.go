package external

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strings"

	"github.com/kindbrave/claude-knowledger/internal/config"
	"github.com/kindbrave/claude-knowledger/internal/core"
)

// Provider runs an external lint tool and parses its output into Findings.
type Provider struct {
	source config.SourceConfig
}

func New(source config.SourceConfig) *Provider {
	return &Provider{source: source}
}

func (p *Provider) Run(ctx context.Context, specID string, changedFiles []string) ([]core.Finding, []core.RuleSet, error) {
	cmdStr := p.source.Command
	if len(changedFiles) > 0 {
		cmdStr = cmdStr + " " + strings.Join(changedFiles, " ")
	}

	parts := strings.Fields(cmdStr)
	if len(parts) == 0 {
		return nil, nil, fmt.Errorf("empty command for spec %s", specID)
	}

	cmd := exec.CommandContext(ctx, parts[0], parts[1:]...)
	if p.source.WorkingDir != "" {
		cmd.Dir = p.source.WorkingDir
	}

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	// Non-zero exit is normal for linters; don't treat it as a fatal error.
	_ = cmd.Run()

	parser := p.source.Parser
	if parser == "" {
		parser = "generic-json"
	}

	findings, err := Parse(parser, specID, stdout.Bytes())
	if err != nil {
		return nil, nil, fmt.Errorf("spec %s: parse error: %w", specID, err)
	}
	return findings, nil, nil
}
