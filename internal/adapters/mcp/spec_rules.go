package mcp

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/kindbrave/claude-knowledger/internal/config"
	"github.com/kindbrave/claude-knowledger/internal/core"
	kbspec "github.com/kindbrave/claude-knowledger/internal/spec/kb"
	mcpgo "github.com/mark3labs/mcp-go/mcp"
)

type listSpecRulesInput struct {
	Scope string `json:"scope,omitempty"`
}

type listSpecRulesResult struct {
	Specifications []config.SpecificationConfig `json:"specifications"`
	RuleSets       []core.RuleSet               `json:"rule_sets"`
}

func (s *Server) handleListSpecRules(ctx context.Context, req mcpgo.CallToolRequest) (*mcpgo.CallToolResult, error) {
	if s.svc == nil {
		return mcpgo.NewToolResultError("service is not configured"), nil
	}
	var input listSpecRulesInput
	if err := req.BindArguments(&input); err != nil {
		return mcpgo.NewToolResultErrorFromErr("invalid arguments", err), nil
	}
	scope, err := s.defaultScope(input.Scope)
	if err != nil {
		return mcpgo.NewToolResultError(err.Error()), nil
	}

	specs := s.svc.ListSpecifications()
	result := listSpecRulesResult{
		Specifications: specs,
		RuleSets:       []core.RuleSet{},
	}
	for _, spec := range specs {
		if !spec.Enabled || spec.Type != "kb" {
			continue
		}
		provider := kbspec.New(s.svc, spec.Source, scope)
		_, ruleSets, err := provider.Run(ctx, spec.ID, nil)
		if err != nil {
			return mcpgo.NewToolResultError(fmt.Sprintf("spec %q: %v", spec.ID, err)), nil
		}
		result.RuleSets = append(result.RuleSets, ruleSets...)
	}

	data, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return mcpgo.NewToolResultError(err.Error()), nil
	}
	return mcpgo.NewToolResultText(string(data)), nil
}
