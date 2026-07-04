package mcp

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/kindbrave/claude-knowledger/internal/config"
	"github.com/kindbrave/claude-knowledger/internal/core"
	"github.com/kindbrave/claude-knowledger/internal/service"
	mcpgo "github.com/mark3labs/mcp-go/mcp"
)

type specGitAddInput struct {
	Scope string   `json:"scope,omitempty"`
	URL   string   `json:"url"`
	ID    string   `json:"id,omitempty"`
	Name  string   `json:"name,omitempty"`
	Tags  []string `json:"tags,omitempty"`
}

type specGitPullInput struct {
	Scope string `json:"scope,omitempty"`
	ID    string `json:"id"`
}

func (s *Server) handleSpecGitAdd(ctx context.Context, request mcpgo.CallToolRequest) (*mcpgo.CallToolResult, error) {
	if s.svc == nil {
		return mcpgo.NewToolResultError("service is not configured"), nil
	}
	var input specGitAddInput
	if err := request.BindArguments(&input); err != nil {
		return mcpgo.NewToolResultErrorFromErr("invalid arguments", err), nil
	}
	url := strings.TrimSpace(input.URL)
	if url == "" {
		return mcpgo.NewToolResultError("url is required"), nil
	}
	id := strings.TrimSpace(input.ID)
	if id == "" {
		id = gitIDFromURLWithFallback(url, "git-spec")
	}
	name := strings.TrimSpace(input.Name)
	if name == "" {
		name = id
	}
	scope, err := s.defaultScope(input.Scope)
	if err != nil {
		return mcpgo.NewToolResultError(err.Error()), nil
	}
	clonePath, err := gitSpecPath(s.svc, scope, id)
	if err != nil {
		return mcpgo.NewToolResultError(err.Error()), nil
	}
	if _, err := os.Stat(clonePath); err == nil {
		return mcpgo.NewToolResultError(fmt.Sprintf("path already exists: %s", clonePath)), nil
	}
	if err := os.MkdirAll(filepath.Dir(clonePath), 0o755); err != nil {
		return mcpgo.NewToolResultError(err.Error()), nil
	}
	out, err := exec.CommandContext(ctx, "git", "clone", url, clonePath).CombinedOutput()
	if err != nil {
		return mcpgo.NewToolResultError(fmt.Sprintf("git clone failed: %v\n%s", err, out)), nil
	}
	record, err := s.svc.CreateKnowledgeBase(ctx, service.CreateKnowledgeBaseInput{
		Scope:     scope,
		ID:        id,
		Name:      name,
		StoreType: "text",
		Path:      clonePath,
	})
	if err != nil {
		_ = os.RemoveAll(clonePath)
		return mcpgo.NewToolResultError(err.Error()), nil
	}
	spec := config.SpecificationConfig{
		ID:      id,
		Name:    name,
		Type:    "kb",
		Enabled: true,
		Source:  config.SourceConfig{KBID: id, Tags: input.Tags},
	}
	if err := s.svc.AddSpecification(scope, spec); err != nil {
		_ = s.svc.DeleteKnowledgeBase(ctx, scope, id)
		_ = os.RemoveAll(clonePath)
		return mcpgo.NewToolResultError(err.Error()), nil
	}
	return mcpgo.NewToolResultStructuredOnly(map[string]any{
		"knowledge_base": record,
		"spec":           spec,
	}), nil
}

func (s *Server) handleSpecGitPull(ctx context.Context, request mcpgo.CallToolRequest) (*mcpgo.CallToolResult, error) {
	if s.svc == nil {
		return mcpgo.NewToolResultError("service is not configured"), nil
	}
	var input specGitPullInput
	if err := request.BindArguments(&input); err != nil {
		return mcpgo.NewToolResultErrorFromErr("invalid arguments", err), nil
	}
	id := strings.TrimSpace(input.ID)
	scope, err := s.defaultScope(input.Scope)
	if err != nil {
		return mcpgo.NewToolResultError(err.Error()), nil
	}
	var kbPath string
	for _, kb := range s.svc.ListKnowledgeBases() {
		if kb.ID == id && kb.Scope == scope {
			kbPath, _ = kb.StoreConfig["path"].(string)
			break
		}
	}
	if kbPath == "" {
		return mcpgo.NewToolResultError(fmt.Sprintf("spec %q (scope: %s) not found or has no path", id, scope)), nil
	}
	out, err := exec.CommandContext(ctx, "git", "-C", kbPath, "pull").CombinedOutput()
	if err != nil {
		return mcpgo.NewToolResultError(fmt.Sprintf("git pull failed: %v\n%s", err, out)), nil
	}
	return mcpgo.NewToolResultStructuredOnly(map[string]any{
		"output": string(out),
		"id":     id,
		"scope":  scope,
	}), nil
}

func (s *Server) handleSpecGitList(_ context.Context, _ mcpgo.CallToolRequest) (*mcpgo.CallToolResult, error) {
	type entry struct {
		Scope string `json:"scope"`
		ID    string `json:"id"`
		Path  string `json:"path"`
	}
	registered := make(map[string]struct{})
	if s.svc != nil {
		for _, kb := range s.svc.ListKnowledgeBases() {
			if kb.StoreType != "text" {
				continue
			}
			path, _ := kb.StoreConfig["path"].(string)
			if path == "" {
				continue
			}
			clean := filepath.Clean(path)
			if filepath.Base(filepath.Dir(clean)) == "git-spec" {
				registered[clean] = struct{}{}
			}
		}
	}
	var results []entry
	if home, err := os.UserHomeDir(); err == nil {
		dir := filepath.Join(home, ".knowledger", "git-spec")
		if entries, err := os.ReadDir(dir); err == nil {
			for _, e := range entries {
				if !e.IsDir() {
					continue
				}
				full := filepath.Join(dir, e.Name())
				if _, ok := registered[filepath.Clean(full)]; ok {
					results = append(results, entry{"global", e.Name(), full})
				}
			}
		}
	}
	if s.svc != nil && s.svc.HasProjectScope() {
		dir := filepath.Join(s.svc.ProjectRoot(), ".knowledger", "git-spec")
		if entries, err := os.ReadDir(dir); err == nil {
			for _, e := range entries {
				if !e.IsDir() {
					continue
				}
				full := filepath.Join(dir, e.Name())
				if _, ok := registered[filepath.Clean(full)]; ok {
					results = append(results, entry{"project", e.Name(), full})
				}
			}
		}
	}
	if results == nil {
		results = []entry{}
	}
	return mcpgo.NewToolResultStructuredOnly(map[string]any{"spec_git_bases": results}), nil
}

func gitSpecPath(svc *service.Service, scope, id string) (string, error) {
	if scope == core.ScopeProject {
		root := svc.ProjectRoot()
		if root == "" {
			return "", fmt.Errorf("not in a project directory")
		}
		return filepath.Join(root, ".knowledger", "git-spec", id), nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".knowledger", "git-spec", id), nil
}
