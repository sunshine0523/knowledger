package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/kindbrave/claude-knowledger/internal/core"
	"github.com/kindbrave/claude-knowledger/internal/service"
	mcpgo "github.com/mark3labs/mcp-go/mcp"
)

type searchKnowledgeInput struct {
	Query      string            `json:"query"`
	KBIDs      []json.RawMessage `json:"kb_ids,omitempty"`
	Scope      string            `json:"scope,omitempty"`
	Limit      int               `json:"limit,omitempty"`
	SearchMode string            `json:"search_mode,omitempty"`
}

type searchKnowledgeResult struct {
	Hits     []searchKnowledgeHit `json:"hits"`
	Warnings []string             `json:"warnings,omitempty"`
}

type searchKnowledgeHit struct {
	ItemID        string         `json:"item_id"`
	KBID          string         `json:"kb_id"`
	Scope         string         `json:"scope"`
	ItemType      string         `json:"item_type,omitempty"`
	Title         string         `json:"title"`
	Snippet       string         `json:"snippet,omitempty"`
	Score         float64        `json:"score"`
	MatchMode     string         `json:"match_mode,omitempty"`
	SourceBackend string         `json:"source_backend,omitempty"`
	Locator       string         `json:"locator,omitempty"`
	Metadata      map[string]any `json:"metadata,omitempty"`
}

func (s *Server) handleSearchKnowledge(ctx context.Context, request mcpgo.CallToolRequest) (*mcpgo.CallToolResult, error) {
	if s.svc == nil {
		return mcpgo.NewToolResultError("service is not configured"), nil
	}
	var input searchKnowledgeInput
	if err := request.BindArguments(&input); err != nil {
		return mcpgo.NewToolResultErrorFromErr("invalid arguments", err), nil
	}
	limit := input.Limit
	if limit == 0 {
		limit = 10
	}
	defaultScope, err := s.defaultScope(input.Scope)
	if err != nil {
		return mcpgo.NewToolResultError(err.Error()), nil
	}
	refs, err := parseSearchKBIDs(input.KBIDs, defaultScope)
	if err != nil {
		return mcpgo.NewToolResultError(err.Error()), nil
	}
	result, err := s.svc.Search(ctx, core.SearchOptions{Query: input.Query, KBIDs: refs, Limit: limit, SearchMode: input.SearchMode})
	if err != nil {
		return mcpgo.NewToolResultError(err.Error()), nil
	}
	return mcpgo.NewToolResultStructuredOnly(toSearchKnowledgeResult(result)), nil
}

func toSearchKnowledgeResult(result service.SearchResult) searchKnowledgeResult {
	hits := make([]searchKnowledgeHit, 0, len(result.Hits))
	for _, hit := range result.Hits {
		hits = append(hits, searchKnowledgeHit{
			ItemID:        hit.ItemID,
			KBID:          hit.KBID,
			Scope:         hit.Scope,
			ItemType:      hit.ItemType,
			Title:         hit.Title,
			Snippet:       hit.Snippet,
			Score:         hit.Score,
			MatchMode:     hit.MatchMode,
			SourceBackend: hit.SourceBackend,
			Locator:       hit.Locator,
			Metadata:      hit.Metadata,
		})
	}
	return searchKnowledgeResult{Hits: hits, Warnings: result.Warnings}
}

func parseSearchKBIDs(raws []json.RawMessage, defaultScope string) ([]core.ScopedKBRef, error) {
	out := make([]core.ScopedKBRef, 0, len(raws))
	for _, raw := range raws {
		var asString string
		if err := json.Unmarshal(raw, &asString); err == nil {
			id := strings.TrimSpace(asString)
			if id == "" {
				continue
			}
			if strings.Contains(id, ":") {
				parts := strings.SplitN(id, ":", 2)
				scope, err := core.NormalizeScope(parts[0])
				if err != nil {
					return nil, fmt.Errorf("kb_ids %q: %w", id, err)
				}
				idPart := strings.TrimSpace(parts[1])
				if idPart == "" {
					return nil, fmt.Errorf("kb_ids %q: id is empty", id)
				}
				out = append(out, core.ScopedKBRef{Scope: scope, ID: idPart})
				continue
			}
			out = append(out, core.ScopedKBRef{Scope: defaultScope, ID: id})
			continue
		}

		var asObj struct {
			Scope string `json:"scope"`
			ID    string `json:"id"`
		}
		if err := json.Unmarshal(raw, &asObj); err != nil {
			return nil, fmt.Errorf("kb_ids element must be string or object: %w", err)
		}
		id := strings.TrimSpace(asObj.ID)
		if id == "" {
			continue
		}
		scope := strings.TrimSpace(asObj.Scope)
		if scope == "" {
			scope = defaultScope
		} else {
			normalized, err := core.NormalizeScope(scope)
			if err != nil {
				return nil, fmt.Errorf("kb_ids[%s]: %w", id, err)
			}
			scope = normalized
		}
		out = append(out, core.ScopedKBRef{Scope: scope, ID: id})
	}
	return out, nil
}
