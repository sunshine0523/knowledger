package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/kindbrave/claude-knowledger/internal/config"
	"github.com/kindbrave/claude-knowledger/internal/core"
	"github.com/kindbrave/claude-knowledger/internal/service"
	"github.com/spf13/cobra"
)

func newSpecGitAddCommand(svc *service.Service) *cobra.Command {
	var id, name string
	var tags []string
	cmd := &cobra.Command{
		Use:   "spec-git-add <url>",
		Short: "Clone a git repository as a kb-type specification (rule set)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if svc == nil {
				return fmt.Errorf("service is not configured")
			}
			url := args[0]
			if id == "" {
				id = gitIDFromURL(url, "git-spec")
			}
			if name == "" {
				name = id
			}
			scope, err := EffectiveScope(ScopeFlagValue(), svc.HasProjectScope())
			if err != nil {
				return err
			}
			clonePath, err := gitSpecPath(svc, scope, id)
			if err != nil {
				return err
			}
			if _, err := os.Stat(clonePath); err == nil {
				return fmt.Errorf("path already exists: %s", clonePath)
			}
			if err := os.MkdirAll(filepath.Dir(clonePath), 0o755); err != nil {
				return err
			}
			out, err := exec.CommandContext(cmd.Context(), "git", "clone", url, clonePath).CombinedOutput()
			if err != nil {
				return fmt.Errorf("git clone failed: %w\n%s", err, out)
			}
			record, err := svc.CreateKnowledgeBase(context.Background(), service.CreateKnowledgeBaseInput{
				Scope:     scope,
				ID:        id,
				Name:      name,
				StoreType: "text",
				Path:      clonePath,
			})
			if err != nil {
				_ = os.RemoveAll(clonePath)
				return err
			}
			spec := config.SpecificationConfig{
				ID:      id,
				Name:    name,
				Type:    "kb",
				Enabled: true,
				Source:  config.SourceConfig{KBID: id, Tags: tags},
			}
			if err := svc.AddSpecification(scope, spec); err != nil {
				_ = svc.DeleteKnowledgeBase(context.Background(), scope, id)
				_ = os.RemoveAll(clonePath)
				return err
			}
			return json.NewEncoder(cmd.OutOrStdout()).Encode(map[string]any{
				"knowledge_base": record,
				"spec":           spec,
			})
		},
	}
	cmd.Flags().StringVar(&id, "id", "", "specification id (derived from repository name if omitted)")
	cmd.Flags().StringVar(&name, "name", "", "human-readable name (defaults to id)")
	cmd.Flags().StringArrayVar(&tags, "tag", nil, "spec tag filter (repeatable, restricts which KB items participate)")
	return cmd
}

func newSpecGitPullCommand(svc *service.Service) *cobra.Command {
	var id string
	cmd := &cobra.Command{
		Use:   "spec-git-pull",
		Short: "Pull latest changes for a spec-git specification",
		RunE: func(cmd *cobra.Command, args []string) error {
			if svc == nil {
				return fmt.Errorf("service is not configured")
			}
			scope, err := EffectiveScope(ScopeFlagValue(), svc.HasProjectScope())
			if err != nil {
				return err
			}
			var kbPath string
			for _, kb := range svc.ListKnowledgeBases() {
				if kb.ID == id && kb.Scope == scope {
					kbPath, _ = kb.StoreConfig["path"].(string)
					break
				}
			}
			if kbPath == "" {
				return fmt.Errorf("spec %q (scope: %s) not found or has no path", id, scope)
			}
			out, err := exec.CommandContext(cmd.Context(), "git", "-C", kbPath, "pull").CombinedOutput()
			fmt.Fprint(cmd.OutOrStdout(), string(out))
			if err != nil {
				return fmt.Errorf("git pull failed: %w", err)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&id, "id", "", "specification id")
	_ = cmd.MarkFlagRequired("id")
	return cmd
}

func newSpecGitListCommand(svc *service.Service) *cobra.Command {
	return &cobra.Command{
		Use:   "spec-git-list",
		Short: "List all spec-git specifications (global and project)",
		RunE: func(cmd *cobra.Command, args []string) error {
			type entry struct {
				Scope string `json:"scope"`
				ID    string `json:"id"`
				Path  string `json:"path"`
			}
			registered := make(map[string]struct{})
			if svc != nil {
				for _, kb := range svc.ListKnowledgeBases() {
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
			if svc != nil && svc.HasProjectScope() {
				dir := filepath.Join(svc.ProjectRoot(), ".knowledger", "git-spec")
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
			return json.NewEncoder(cmd.OutOrStdout()).Encode(results)
		},
	}
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
