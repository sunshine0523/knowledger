package mcp

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/kindbrave/claude-knowledger/internal/config"
	"github.com/kindbrave/claude-knowledger/internal/core"
	mcpgo "github.com/mark3labs/mcp-go/mcp"
)

type listSpecificationsInput struct{}

type runLintInput struct {
	ChangedFiles []string `json:"changed_files,omitempty"`
	SpecIDs      []string `json:"spec_ids,omitempty"`
}

type addSpecificationInput struct {
	Scope        string   `json:"scope,omitempty"`
	ID           string   `json:"id"`
	Name         string   `json:"name,omitempty"`
	Type         string   `json:"type"`
	Enabled      *bool    `json:"enabled,omitempty"`
	KBID         string   `json:"kb_id,omitempty"`
	Tags         []string `json:"tags,omitempty"`
	Command      string   `json:"command,omitempty"`
	Parser       string   `json:"parser,omitempty"`
	WorkingDir   string   `json:"working_dir,omitempty"`
	Script       string   `json:"script,omitempty"`
	OutputFormat string   `json:"output_format,omitempty"`
}

type deleteSpecificationInput struct {
	Scope string `json:"scope,omitempty"`
	ID    string `json:"id"`
}

type addRuleToSpecInput struct {
	SpecID  string   `json:"spec_id"`
	Title   string   `json:"title"`
	Content string   `json:"content"`
	Tags    []string `json:"tags,omitempty"`
	Scope   string   `json:"scope,omitempty"`
}

func (s *Server) registerSpecTools() {
	listSpecRulesTool := mcpgo.NewTool(
		"list_spec_rules",
		mcpgo.WithDescription("列出所有规范（specifications）并返回每个已启用 kb-type 规范的完整规则条目。在任何涉及编码、设计或约定的任务开始时必须调用——返回的规则全部强制执行，不做相关性过滤。不执行 external/script 类型的 linter（如需 lint 结果请用 run_lint）。纯只读，一次调用拿全量。通用知识（Q&A、决策、调试配方）请用 search_knowledge，本工具只用于可执行规范。"),
		mcpgo.WithString("scope", mcpgo.Description("Specification scope: project or global. 省略则返回所有 scope。"), mcpgo.Enum("project", "global")),
		mcpgo.WithReadOnlyHintAnnotation(true),
		mcpgo.WithDestructiveHintAnnotation(false),
		mcpgo.WithIdempotentHintAnnotation(true),
		mcpgo.WithOpenWorldHintAnnotation(false),
	)
	listSpecTool := mcpgo.NewTool(
		"list_specifications",
		mcpgo.WithDescription("List all configured specifications (id, name, type, enabled)."),
		mcpgo.WithReadOnlyHintAnnotation(true),
		mcpgo.WithDestructiveHintAnnotation(false),
		mcpgo.WithIdempotentHintAnnotation(true),
		mcpgo.WithOpenWorldHintAnnotation(false),
	)
	runLintTool := mcpgo.NewTool(
		"run_lint",
		mcpgo.WithDescription("Run specification checks and return a LintResult JSON with findings and rule sets."),
		mcpgo.WithArray("changed_files", mcpgo.Description("Files to check. Omit to scan all."), mcpgo.WithStringItems()),
		mcpgo.WithArray("spec_ids", mcpgo.Description("Spec IDs to run. Omit for all enabled specs."), mcpgo.WithStringItems()),
		mcpgo.WithReadOnlyHintAnnotation(false),
		mcpgo.WithDestructiveHintAnnotation(false),
		mcpgo.WithIdempotentHintAnnotation(false),
		mcpgo.WithOpenWorldHintAnnotation(false),
	)
	addSpecTool := mcpgo.NewTool(
		"add_specification",
		mcpgo.WithDescription("Add a new specification (kb, external, or script type). Persists to specs.json under the resolved scope."),
		mcpgo.WithString("scope", mcpgo.Description("Specification scope: project or global. Defaults to project when running in a project directory, else global."), mcpgo.Enum("project", "global")),
		mcpgo.WithString("id", mcpgo.Description("Unique specification ID"), mcpgo.Required()),
		mcpgo.WithString("type", mcpgo.Description("Specification type: kb, external, or script"), mcpgo.Required()),
		mcpgo.WithString("name", mcpgo.Description("Specification display name")),
		mcpgo.WithBoolean("enabled", mcpgo.Description("Whether the specification is enabled (default true)")),
		mcpgo.WithString("kb_id", mcpgo.Description("KB ID (for kb type)")),
		mcpgo.WithArray("tags", mcpgo.Description("Tags (for kb type)"), mcpgo.WithStringItems()),
		mcpgo.WithString("command", mcpgo.Description("Command to run (for external type)")),
		mcpgo.WithString("parser", mcpgo.Description("Output parser: golangci-lint, checkstyle, eslint, generic-json (for external type)")),
		mcpgo.WithString("working_dir", mcpgo.Description("Working directory (for external type)")),
		mcpgo.WithString("script", mcpgo.Description("Script path (for script type)")),
		mcpgo.WithString("output_format", mcpgo.Description("Output format: json or text (for script type)")),
		mcpgo.WithReadOnlyHintAnnotation(false),
		mcpgo.WithDestructiveHintAnnotation(false),
		mcpgo.WithIdempotentHintAnnotation(false),
		mcpgo.WithOpenWorldHintAnnotation(false),
	)
	deleteSpecTool := mcpgo.NewTool(
		"delete_specification",
		mcpgo.WithDescription("Delete a runtime specification by ID. Static specifications from knowledger.yaml cannot be deleted."),
		mcpgo.WithString("scope", mcpgo.Description("Scope to delete from: project or global. Defaults to project when in a project directory, else global."), mcpgo.Enum("project", "global")),
		mcpgo.WithString("id", mcpgo.Description("Specification ID to delete"), mcpgo.Required()),
		mcpgo.WithReadOnlyHintAnnotation(false),
		mcpgo.WithDestructiveHintAnnotation(true),
		mcpgo.WithIdempotentHintAnnotation(false),
		mcpgo.WithOpenWorldHintAnnotation(false),
	)
	addRuleToSpecTool := mcpgo.NewTool(
		"add_rule_to_spec",
		mcpgo.WithDescription("Add a rule item to a kb-type specification's backing knowledge base."),
		mcpgo.WithString("spec_id", mcpgo.Description("Specification ID (must be kb type)"), mcpgo.Required()),
		mcpgo.WithString("title", mcpgo.Description("Rule title"), mcpgo.Required()),
		mcpgo.WithString("content", mcpgo.Description("Rule content"), mcpgo.Required()),
		mcpgo.WithArray("tags", mcpgo.Description("Rule tags"), mcpgo.WithStringItems()),
		mcpgo.WithString("scope", mcpgo.Description("Scope: global or project (default: global)")),
		mcpgo.WithReadOnlyHintAnnotation(false),
		mcpgo.WithDestructiveHintAnnotation(false),
		mcpgo.WithIdempotentHintAnnotation(false),
		mcpgo.WithOpenWorldHintAnnotation(false),
	)

	s.tools = append(s.tools, listSpecRulesTool, listSpecTool, runLintTool, addSpecTool, deleteSpecTool, addRuleToSpecTool)
	s.server.AddTool(listSpecRulesTool, s.handleListSpecRules)
	s.server.AddTool(listSpecTool, s.handleListSpecifications)
	s.server.AddTool(runLintTool, s.handleRunLint)
	s.server.AddTool(addSpecTool, s.handleAddSpecification)
	s.server.AddTool(deleteSpecTool, s.handleDeleteSpecification)
	s.server.AddTool(addRuleToSpecTool, s.handleAddRuleToSpec)
}

func (s *Server) handleListSpecifications(_ context.Context, _ mcpgo.CallToolRequest) (*mcpgo.CallToolResult, error) {
	if s.svc == nil {
		return mcpgo.NewToolResultError("service is not configured"), nil
	}
	specs := s.svc.ListSpecifications()
	data, err := json.MarshalIndent(specs, "", "  ")
	if err != nil {
		return mcpgo.NewToolResultError(err.Error()), nil
	}
	return mcpgo.NewToolResultText(string(data)), nil
}

func (s *Server) handleRunLint(ctx context.Context, request mcpgo.CallToolRequest) (*mcpgo.CallToolResult, error) {
	if s.svc == nil {
		return mcpgo.NewToolResultError("service is not configured"), nil
	}
	var input runLintInput
	if err := request.BindArguments(&input); err != nil {
		return mcpgo.NewToolResultErrorFromErr("invalid arguments", err), nil
	}
	result := s.svc.RunLint(ctx, input.ChangedFiles, input.SpecIDs)
	data, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return mcpgo.NewToolResultError(err.Error()), nil
	}
	return mcpgo.NewToolResultText(strings.TrimSpace(string(data))), nil
}

func (s *Server) handleAddSpecification(_ context.Context, request mcpgo.CallToolRequest) (*mcpgo.CallToolResult, error) {
	if s.svc == nil {
		return mcpgo.NewToolResultError("service is not configured"), nil
	}
	var input addSpecificationInput
	if err := request.BindArguments(&input); err != nil {
		return mcpgo.NewToolResultErrorFromErr("invalid arguments", err), nil
	}
	if input.ID == "" {
		return mcpgo.NewToolResultError("id is required"), nil
	}
	if input.Type == "" {
		return mcpgo.NewToolResultError("type is required"), nil
	}
	enabled := true
	if input.Enabled != nil {
		enabled = *input.Enabled
	}
	spec := config.SpecificationConfig{
		ID:      input.ID,
		Name:    input.Name,
		Type:    input.Type,
		Enabled: enabled,
		Source: config.SourceConfig{
			KBID:         input.KBID,
			Tags:         input.Tags,
			Command:      input.Command,
			Parser:       input.Parser,
			WorkingDir:   input.WorkingDir,
			Script:       input.Script,
			OutputFormat: input.OutputFormat,
		},
	}
	scope, err := s.defaultScope(input.Scope)
	if err != nil {
		return mcpgo.NewToolResultError(err.Error()), nil
	}
	if err := s.svc.AddSpecification(scope, spec); err != nil {
		return mcpgo.NewToolResultError(err.Error()), nil
	}
	return mcpgo.NewToolResultText("specification \"" + input.ID + "\" added"), nil
}

func (s *Server) handleDeleteSpecification(_ context.Context, request mcpgo.CallToolRequest) (*mcpgo.CallToolResult, error) {
	if s.svc == nil {
		return mcpgo.NewToolResultError("service is not configured"), nil
	}
	var input deleteSpecificationInput
	if err := request.BindArguments(&input); err != nil {
		return mcpgo.NewToolResultErrorFromErr("invalid arguments", err), nil
	}
	if input.ID == "" {
		return mcpgo.NewToolResultError("id is required"), nil
	}
	scope, err := s.defaultScope(input.Scope)
	if err != nil {
		return mcpgo.NewToolResultError(err.Error()), nil
	}
	if err := s.svc.DeleteSpecification(scope, input.ID); err != nil {
		return mcpgo.NewToolResultError(err.Error()), nil
	}
	return mcpgo.NewToolResultText("specification \"" + input.ID + "\" deleted"), nil
}

func (s *Server) handleAddRuleToSpec(ctx context.Context, request mcpgo.CallToolRequest) (*mcpgo.CallToolResult, error) {
	if s.svc == nil {
		return mcpgo.NewToolResultError("service is not configured"), nil
	}
	var input addRuleToSpecInput
	if err := request.BindArguments(&input); err != nil {
		return mcpgo.NewToolResultErrorFromErr("invalid arguments", err), nil
	}
	specs := s.svc.ListSpecifications()
	var kbID string
	for _, sp := range specs {
		if sp.ID == input.SpecID && sp.Type == "kb" {
			kbID = sp.Source.KBID
			break
		}
	}
	if kbID == "" {
		return mcpgo.NewToolResultError("spec \"" + input.SpecID + "\" not found or is not a kb-type specification"), nil
	}
	scope := input.Scope
	if scope == "" {
		scope = "global"
	}
	_, _, _, err := s.svc.Add(ctx, core.AddInput{
		Scope:   scope,
		KBID:    kbID,
		Title:   input.Title,
		Content: input.Content,
		Tags:    input.Tags,
	})
	if err != nil {
		return mcpgo.NewToolResultError(err.Error()), nil
	}
	return mcpgo.NewToolResultText("rule added to spec \"" + input.SpecID + "\" (kb: " + kbID + ")"), nil
}
