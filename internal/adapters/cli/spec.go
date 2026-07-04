package cli

import (
	"encoding/json"
	"fmt"

	"github.com/kindbrave/claude-knowledger/internal/config"
	"github.com/kindbrave/claude-knowledger/internal/core"
	"github.com/kindbrave/claude-knowledger/internal/service"
	"github.com/spf13/cobra"
)

func newSpecCommand(svc *service.Service) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "spec",
		Short: "Manage specifications",
	}
	cmd.AddCommand(newSpecListCommand(svc))
	cmd.AddCommand(newSpecAddCommand(svc))
	cmd.AddCommand(newSpecDeleteCommand(svc))
	cmd.AddCommand(newSpecAddRuleCommand(svc))
	return cmd
}

func newSpecListCommand(svc *service.Service) *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List all configured specifications",
		RunE: func(cmd *cobra.Command, args []string) error {
			if svc == nil {
				return fmt.Errorf("service is not configured")
			}
			specs := svc.ListSpecifications()
			data, err := json.MarshalIndent(specs, "", "  ")
			if err != nil {
				return err
			}
			fmt.Fprintln(cmd.OutOrStdout(), string(data))
			return nil
		},
	}
}

func newSpecAddRuleCommand(svc *service.Service) *cobra.Command {
	var specID, title, content string
	var tags []string

	cmd := &cobra.Command{
		Use:   "add-rule",
		Short: "Add a rule item to a kb-type specification's backing knowledge base",
		RunE: func(cmd *cobra.Command, args []string) error {
			if svc == nil {
				return fmt.Errorf("service is not configured")
			}
			specs := svc.ListSpecifications()
			var kbID string
			for _, s := range specs {
				if s.ID == specID && s.Type == "kb" {
					kbID = s.Source.KBID
					break
				}
			}
			if kbID == "" {
				return fmt.Errorf("spec %q not found or is not a kb-type specification", specID)
			}
			scope := scopeFlag
			if scope == "" {
				scope = "global"
			}
			_, _, _, err := svc.Add(cmd.Context(), core.AddInput{
				Scope:   scope,
				KBID:    kbID,
				Title:   title,
				Content: content,
				Tags:    tags,
			})
			if err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "rule added to spec %q (kb: %s)\n", specID, kbID)
			return nil
		},
	}
	cmd.Flags().StringVar(&specID, "spec", "", "Specification ID (required)")
	cmd.Flags().StringVar(&title, "title", "", "Rule title (required)")
	cmd.Flags().StringVar(&content, "content", "", "Rule content (required)")
	cmd.Flags().StringArrayVar(&tags, "tag", nil, "Rule tags (repeatable)")
	_ = cmd.MarkFlagRequired("spec")
	_ = cmd.MarkFlagRequired("title")
	_ = cmd.MarkFlagRequired("content")
	return cmd
}

func newSpecAddCommand(svc *service.Service) *cobra.Command {
	var id, name, specType string
	var enabled bool
	var kbID string
	var tags []string
	var command, parser, workingDir string
	var script, outputFormat string

	cmd := &cobra.Command{
		Use:   "add",
		Short: "Add a new specification",
		RunE: func(cmd *cobra.Command, args []string) error {
			if svc == nil {
				return fmt.Errorf("service is not configured")
			}
			spec := config.SpecificationConfig{
				ID:      id,
				Name:    name,
				Type:    specType,
				Enabled: enabled,
				Source: config.SourceConfig{
					KBID:         kbID,
					Tags:         tags,
					Command:      command,
					Parser:       parser,
					WorkingDir:   workingDir,
					Script:       script,
					OutputFormat: outputFormat,
				},
			}
			scope, err := EffectiveScope(ScopeFlagValue(), svc.HasProjectScope())
			if err != nil {
				return err
			}
			if err := svc.AddSpecification(scope, spec); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "specification %q added\n", id)
			return nil
		},
	}
	cmd.Flags().StringVar(&id, "id", "", "Specification ID (required)")
	cmd.Flags().StringVar(&name, "name", "", "Specification name")
	cmd.Flags().StringVar(&specType, "type", "", "Specification type: kb, external, script (required)")
	cmd.Flags().BoolVar(&enabled, "enabled", true, "Whether the specification is enabled")
	cmd.Flags().StringVar(&kbID, "kb-id", "", "KB ID (for kb type)")
	cmd.Flags().StringArrayVar(&tags, "tag", nil, "Tags (for kb type, repeatable)")
	cmd.Flags().StringVar(&command, "command", "", "Command (for external type)")
	cmd.Flags().StringVar(&parser, "parser", "", "Parser (for external type)")
	cmd.Flags().StringVar(&workingDir, "working-dir", "", "Working directory (for external type)")
	cmd.Flags().StringVar(&script, "script", "", "Script path (for script type)")
	cmd.Flags().StringVar(&outputFormat, "output-format", "", "Output format (for script type)")
	_ = cmd.MarkFlagRequired("id")
	_ = cmd.MarkFlagRequired("type")
	return cmd
}

func newSpecDeleteCommand(svc *service.Service) *cobra.Command {
	return &cobra.Command{
		Use:   "delete <spec-id>",
		Short: "Delete a specification",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if svc == nil {
				return fmt.Errorf("service is not configured")
			}
			scope, err := EffectiveScope(ScopeFlagValue(), svc.HasProjectScope())
			if err != nil {
				return err
			}
			if err := svc.DeleteSpecification(scope, args[0]); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "specification %q deleted\n", args[0])
			return nil
		},
	}
}
