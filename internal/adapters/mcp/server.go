package mcp

import (
	"context"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/kindbrave/claude-knowledger/internal/core"
	"github.com/kindbrave/claude-knowledger/internal/service"
	mcpgo "github.com/mark3labs/mcp-go/mcp"
	mcpserver "github.com/mark3labs/mcp-go/server"
)

const serverName = "knowledger"
const serverVersion = "0.1.0"

// serverInstructions is injected into the MCP `initialize` response and
// surfaced by hosts (e.g. Claude Code) inside the system prompt. This
// is the strongest priming channel — stronger than per-tool descriptions,
// because the model reads it every turn without having to scan tool
// metadata. Keep it sharp, behavioral, and bilingual where the user base
// asks questions in mixed languages.
const serverInstructions = `# Knowledger — typed knowledge bases, runs BEFORE answering

Every collection is a knowledge base with type ` + "`knowledge`" + ` or
` + "`specification`" + `. Both use the same knowledge-base and item tools.

## Rules and knowledge

BEFORE answering, writing code, designing, or making any technical recommendation:

1. Call ` + "`list_knowledge_bases`" + ` and identify every enabled
   ` + "`specification`" + ` knowledge base.
2. Call ` + "`list_knowledge_items`" + ` for each specification KB and fetch every full body
   with ` + "`get_knowledge_item`" + `. Every specification applies; do not relevance-filter them.
3. Call ` + "`search_knowledge`" + ` for relevant items from ` + "`knowledge`" + ` KBs and
   fetch applicable full bodies with ` + "`get_knowledge_item`" + `.

## Capture — only on explicit user intent

` + "`add_knowledge_item`" + ` when the user says save / capture / remember /
记一下 / 保存到 / 添加到 — and the target KB is unambiguous.
Otherwise ` + "`list_knowledge_bases`" + ` and ask which KB to use.

## Skip

Conversational turns, ephemeral state, secrets, or anything fully
derivable from the current diff/file.
`

type Server struct {
	svc    *service.Service
	server *mcpserver.MCPServer
	tools  []mcpgo.Tool
}

type ToolForTest = mcpgo.Tool

type getKnowledgeItemInput struct {
	Scope  string `json:"scope,omitempty"`
	KBID   string `json:"kb_id"`
	ItemID string `json:"item_id"`
}

type listKnowledgeItemsInput struct {
	Scope  string `json:"scope,omitempty"`
	KBID   string `json:"kb_id"`
	Limit  int    `json:"limit,omitempty"`
	Offset int    `json:"offset,omitempty"`
}

// knowledgeItemSummary is the lean view returned by list_knowledge_items —
// no Content, so a large KB can be browsed cheaply as a directory.
type knowledgeItemSummary struct {
	ID        string         `json:"id"`
	KBID      string         `json:"kb_id"`
	Scope     string         `json:"scope"`
	Type      string         `json:"type,omitempty"`
	Title     string         `json:"title"`
	Summary   string         `json:"summary,omitempty"`
	Metadata  map[string]any `json:"metadata,omitempty"`
	Tags      []string       `json:"tags,omitempty"`
	UpdatedAt string         `json:"updated_at,omitempty"`
}

type listKnowledgeItemsResult struct {
	Items  []knowledgeItemSummary `json:"items"`
	Total  int                    `json:"total"`
	Offset int                    `json:"offset"`
	Limit  int                    `json:"limit"`
}

type addKnowledgeItemInput struct {
	Scope    string         `json:"scope,omitempty"`
	KBID     string         `json:"kb_id"`
	Title    string         `json:"title"`
	Content  string         `json:"content"`
	Tags     []string       `json:"tags,omitempty"`
	Metadata map[string]any `json:"metadata,omitempty"`
}

type deleteKnowledgeItemInput struct {
	Scope  string `json:"scope,omitempty"`
	KBID   string `json:"kb_id"`
	ItemID string `json:"item_id"`
}

type createKnowledgeBaseInput struct {
	Scope           string   `json:"scope,omitempty"`
	ID              string   `json:"id"`
	Type            string   `json:"type,omitempty"`
	Name            string   `json:"name,omitempty"`
	StoreType       string   `json:"store_type"`
	Path            string   `json:"path,omitempty"`
	Enabled         *bool    `json:"enabled,omitempty"`
	SemanticEnabled *bool    `json:"semantic_enabled,omitempty"`
	Tags            []string `json:"tags,omitempty"`
}

type deleteKnowledgeBaseInput struct {
	Scope string `json:"scope,omitempty"`
	ID    string `json:"id"`
}

type indexKnowledgeInput struct {
	Scope   string `json:"scope,omitempty"`
	KBID    string `json:"kb_id,omitempty"`
	Rebuild bool   `json:"rebuild,omitempty"`
}

func NewServer(svc *service.Service) *Server {
	adapter := &Server{svc: svc, server: mcpserver.NewMCPServer(
		serverName,
		serverVersion,
		mcpserver.WithInstructions(serverInstructions),
	)}
	adapter.registerTools()
	return adapter
}

func (s *Server) MCPServer() *mcpserver.MCPServer { return s.server }

func (s *Server) Tools() []mcpgo.Tool { return append([]mcpgo.Tool(nil), s.tools...) }

func (s *Server) ServeStdio() error {
	s.logModeBanner(os.Stderr)
	logger := log.New(os.Stderr, "knowledger mcp: ", log.LstdFlags)
	return mcpserver.ServeStdio(s.server, mcpserver.WithErrorLogger(logger))
}

// logModeBanner writes a single line describing the scope mode the MCP
// server is running in. Surfaced via stderr so the operator can confirm
// the server discovered the project root they expected.
func (s *Server) logModeBanner(w *os.File) {
	if s.svc != nil && s.svc.HasProjectScope() {
		fmt.Fprintf(w, "knowledger: project mode (root=%s)\n", s.svc.ProjectRoot())
		return
	}
	fmt.Fprintln(w, "knowledger: global mode")
}

// LogModeBannerForTest exposes logModeBanner to the external test package.
func (s *Server) LogModeBannerForTest(w *os.File) { s.logModeBanner(w) }

func (s *Server) handleGetKnowledgeItem(ctx context.Context, request mcpgo.CallToolRequest) (*mcpgo.CallToolResult, error) {
	if s.svc == nil {
		return mcpgo.NewToolResultError("service is not configured"), nil
	}
	var input getKnowledgeItemInput
	if err := request.BindArguments(&input); err != nil {
		return mcpgo.NewToolResultErrorFromErr("invalid arguments", err), nil
	}
	scope, err := s.defaultScope(input.Scope)
	if err != nil {
		return mcpgo.NewToolResultError(err.Error()), nil
	}
	item, err := s.svc.GetKnowledgeItem(ctx, scope, input.KBID, input.ItemID)
	if err != nil {
		return mcpgo.NewToolResultError(err.Error()), nil
	}
	return mcpgo.NewToolResultStructuredOnly(item), nil
}

func (s *Server) handleListKnowledgeItems(ctx context.Context, request mcpgo.CallToolRequest) (*mcpgo.CallToolResult, error) {
	if s.svc == nil {
		return mcpgo.NewToolResultError("service is not configured"), nil
	}
	var input listKnowledgeItemsInput
	if err := request.BindArguments(&input); err != nil {
		return mcpgo.NewToolResultErrorFromErr("invalid arguments", err), nil
	}
	scope, err := s.defaultScope(input.Scope)
	if err != nil {
		return mcpgo.NewToolResultError(err.Error()), nil
	}
	items, err := s.svc.ListKnowledgeItems(ctx, scope, input.KBID)
	if err != nil {
		return mcpgo.NewToolResultError(err.Error()), nil
	}
	total := len(items)
	offset := input.Offset
	if offset < 0 {
		offset = 0
	}
	if offset > total {
		offset = total
	}
	end := total
	if input.Limit > 0 && offset+input.Limit < end {
		end = offset + input.Limit
	}
	page := items[offset:end]
	var b strings.Builder
	for i, item := range page {
		if i > 0 {
			b.WriteByte('\n')
		}
		fmt.Fprintf(&b, "- %s\t%s", item.ID, item.Title)
		if item.Summary != "" {
			fmt.Fprintf(&b, "\n  Summary: %s", item.Summary)
		}
		if len(item.Metadata) > 0 {
			b.WriteString("\n  Metadata:")
			for k, v := range item.Metadata {
				fmt.Fprintf(&b, "\n    %s: %v", k, v)
			}
		}
		if len(item.Tags) > 0 {
			fmt.Fprintf(&b, "\n  Tags: [%s]", strings.Join(item.Tags, ", "))
		}
		if !item.UpdatedAt.IsZero() {
			fmt.Fprintf(&b, "\n  Updated: %s", item.UpdatedAt.UTC().Format("2006-01-02T15:04:05Z"))
		}
	}
	footer := fmt.Sprintf("\n\nTotal: %d items, showing %d-%d", total, offset+1, offset+len(page))
	return mcpgo.NewToolResultText(b.String() + footer), nil
}

func (s *Server) handleAddKnowledgeItem(ctx context.Context, request mcpgo.CallToolRequest) (*mcpgo.CallToolResult, error) {
	if s.svc == nil {
		return mcpgo.NewToolResultError("service is not configured"), nil
	}
	var input addKnowledgeItemInput
	if err := request.BindArguments(&input); err != nil {
		return mcpgo.NewToolResultErrorFromErr("invalid arguments", err), nil
	}
	scope, err := s.defaultScope(input.Scope)
	if err != nil {
		return mcpgo.NewToolResultError(err.Error()), nil
	}
	item, ingestionResult, indexStatus, err := s.svc.Add(ctx, core.AddInput{KBID: input.KBID, Scope: scope, Title: input.Title, Content: input.Content, Tags: input.Tags, Metadata: input.Metadata})
	if err != nil {
		return mcpgo.NewToolResultError(err.Error()), nil
	}
	return mcpgo.NewToolResultStructuredOnly(map[string]any{"item": item, "ingestion_result": ingestionResult, "index_status": indexStatus}), nil
}

func (s *Server) handleDeleteKnowledgeItem(ctx context.Context, request mcpgo.CallToolRequest) (*mcpgo.CallToolResult, error) {
	if s.svc == nil {
		return mcpgo.NewToolResultError("service is not configured"), nil
	}
	var input deleteKnowledgeItemInput
	if err := request.BindArguments(&input); err != nil {
		return mcpgo.NewToolResultErrorFromErr("invalid arguments", err), nil
	}
	scope, err := s.defaultScope(input.Scope)
	if err != nil {
		return mcpgo.NewToolResultError(err.Error()), nil
	}
	if err := s.svc.DeleteKnowledgeItem(ctx, scope, input.KBID, input.ItemID); err != nil {
		return mcpgo.NewToolResultError(err.Error()), nil
	}
	return mcpgo.NewToolResultStructuredOnly(map[string]any{
		"deleted": true,
		"scope":   scope,
		"kb_id":   strings.TrimSpace(input.KBID),
		"item_id": strings.TrimSpace(input.ItemID),
	}), nil
}

func (s *Server) handleListKnowledgeBases(ctx context.Context, request mcpgo.CallToolRequest) (*mcpgo.CallToolResult, error) {
	_ = request
	if s.svc == nil {
		return mcpgo.NewToolResultError("service is not configured"), nil
	}
	kbs := s.svc.ListKnowledgeBases()
	return mcpgo.NewToolResultText(formatKnowledgeBasesWithItems(ctx, s.svc, kbs)), nil
}

func formatKnowledgeBasesWithItems(ctx context.Context, svc *service.Service, kbs []core.KnowledgeBase) string {
	if len(kbs) == 0 {
		return "no knowledge bases configured"
	}
	var b strings.Builder
	for i, kb := range kbs {
		if i > 0 {
			b.WriteByte('\n')
		}
		writeKnowledgeBaseHeader(&b, kb)
		writeKnowledgeBaseItems(ctx, &b, svc, kb)
	}
	return strings.TrimRight(b.String(), "\n")
}

func writeKnowledgeBaseHeader(b *strings.Builder, kb core.KnowledgeBase) {
	scope := kb.Scope
	if scope == "" {
		scope = core.ScopeGlobal
	}
	name := kb.Name
	if name == "" {
		name = kb.ID
	}
	fmt.Fprintf(b, "=== [%s:%s] %s (type=%s, store=%s) ===\n", scope, kb.ID, name, kb.Type, kb.StoreType)
}

func writeKnowledgeBaseItems(ctx context.Context, b *strings.Builder, svc *service.Service, kb core.KnowledgeBase) {
	items, err := svc.ListKnowledgeItems(ctx, kb.Scope, kb.ID)
	if err != nil {
		fmt.Fprintf(b, "Error listing items: %s\n", err.Error())
		return
	}
	if len(items) == 0 {
		b.WriteString("(empty)\n")
		return
	}
	b.WriteString("Item-ID\tTitle\tMetadata\tSummary\n")
	for _, item := range items {
		fmt.Fprintf(b, "%s\t%s\t%s\t%s\n", item.ID, item.Title, formatMetadataInline(item.Metadata), item.Summary)
	}
}

func formatMetadataInline(metadata map[string]any) string {
	if len(metadata) == 0 {
		return "-"
	}
	var parts []string
	for k, v := range metadata {
		parts = append(parts, fmt.Sprintf("%s:%v", k, v))
	}
	return strings.Join(parts, "; ")
}

func (s *Server) handleCreateKnowledgeBase(ctx context.Context, request mcpgo.CallToolRequest) (*mcpgo.CallToolResult, error) {
	if s.svc == nil {
		return mcpgo.NewToolResultError("service is not configured"), nil
	}
	var input createKnowledgeBaseInput
	if err := request.BindArguments(&input); err != nil {
		return mcpgo.NewToolResultErrorFromErr("invalid arguments", err), nil
	}
	scope, err := s.defaultScope(input.Scope)
	if err != nil {
		return mcpgo.NewToolResultError(err.Error()), nil
	}
	record, err := s.svc.CreateKnowledgeBase(ctx, service.CreateKnowledgeBaseInput{
		Scope:           scope,
		ID:              strings.TrimSpace(input.ID),
		Type:            strings.TrimSpace(input.Type),
		Name:            strings.TrimSpace(input.Name),
		StoreType:       strings.TrimSpace(input.StoreType),
		Path:            input.Path,
		Enabled:         input.Enabled,
		SemanticEnabled: input.SemanticEnabled,
		Tags:            input.Tags,
	})
	if err != nil {
		return mcpgo.NewToolResultError(err.Error()), nil
	}
	return mcpgo.NewToolResultStructuredOnly(map[string]any{"knowledge_base": record}), nil
}

func (s *Server) handleDeleteKnowledgeBase(ctx context.Context, request mcpgo.CallToolRequest) (*mcpgo.CallToolResult, error) {
	if s.svc == nil {
		return mcpgo.NewToolResultError("service is not configured"), nil
	}
	var input deleteKnowledgeBaseInput
	if err := request.BindArguments(&input); err != nil {
		return mcpgo.NewToolResultErrorFromErr("invalid arguments", err), nil
	}
	scope, err := s.defaultScope(input.Scope)
	if err != nil {
		return mcpgo.NewToolResultError(err.Error()), nil
	}
	id := strings.TrimSpace(input.ID)
	if id == "" {
		return mcpgo.NewToolResultError("knowledge base id is required"), nil
	}
	// If this is a git-knowledge base, resolve and clean up the local clone
	// directory after removing the KB record.
	clonePath, _ := gitKnowledgePath(s.svc, scope, id)
	if clonePath != "" {
		if info, err := os.Stat(clonePath); err == nil && info.IsDir() {
			if err := s.svc.DeleteKnowledgeBase(ctx, scope, id); err != nil {
				return mcpgo.NewToolResultError(err.Error()), nil
			}
			if err := os.RemoveAll(clonePath); err != nil {
				return mcpgo.NewToolResultError(fmt.Sprintf("knowledge base deleted but failed to remove local directory %s: %v", clonePath, err)), nil
			}
			return mcpgo.NewToolResultStructuredOnly(map[string]any{
				"deleted": true,
				"scope":   scope,
				"id":      id,
				"path":    clonePath,
			}), nil
		}
	}
	if err := s.svc.DeleteKnowledgeBase(ctx, scope, id); err != nil {
		return mcpgo.NewToolResultError(err.Error()), nil
	}
	return mcpgo.NewToolResultStructuredOnly(map[string]any{
		"deleted": true,
		"scope":   scope,
		"id":      id,
	}), nil
}

func (s *Server) handleIndexKnowledge(ctx context.Context, request mcpgo.CallToolRequest) (*mcpgo.CallToolResult, error) {
	if s.svc == nil {
		return mcpgo.NewToolResultError("service is not configured"), nil
	}
	var input indexKnowledgeInput
	if err := request.BindArguments(&input); err != nil {
		return mcpgo.NewToolResultErrorFromErr("invalid arguments", err), nil
	}
	scope := strings.TrimSpace(input.Scope)
	kbID := strings.TrimSpace(input.KBID)
	if scope == "" && kbID != "" {
		// Single-KB index needs a concrete scope; fall through to default.
		var err error
		scope, err = s.defaultScope("")
		if err != nil {
			return mcpgo.NewToolResultError(err.Error()), nil
		}
	} else if scope != "" {
		normalized, err := core.NormalizeScope(scope)
		if err != nil {
			return mcpgo.NewToolResultError(err.Error()), nil
		}
		scope = normalized
	}
	result, err := s.svc.IndexKnowledge(ctx, service.IndexKnowledgeInput{Scope: scope, KBID: kbID, Rebuild: input.Rebuild})
	if err != nil {
		return mcpgo.NewToolResultError(err.Error()), nil
	}
	return mcpgo.NewToolResultStructuredOnly(result), nil
}

// defaultScope normalises the scope from a tool input. An empty string
// resolves to project when the service is in project mode, else global.
func (s *Server) defaultScope(raw string) (string, error) {
	if strings.TrimSpace(raw) == "" {
		if s.svc != nil && s.svc.HasProjectScope() {
			return core.ScopeProject, nil
		}
		return core.ScopeGlobal, nil
	}
	return core.NormalizeScope(raw)
}
