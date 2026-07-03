package cli

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/kindbrave/claude-knowledger/internal/service"
	"github.com/spf13/cobra"
)

func newLintCommand(svc *service.Service) *cobra.Command {
	var changedFiles []string
	var specIDs string

	cmd := &cobra.Command{
		Use:   "lint",
		Short: "Run specification checks and output a LintResult JSON",
		RunE: func(cmd *cobra.Command, args []string) error {
			if svc == nil {
				return fmt.Errorf("service is not configured")
			}
			var ids []string
			if specIDs != "" {
				for _, id := range strings.Split(specIDs, ",") {
					id = strings.TrimSpace(id)
					if id != "" {
						ids = append(ids, id)
					}
				}
			}
			result := svc.RunLint(cmd.Context(), changedFiles, ids)
			data, err := json.MarshalIndent(result, "", "  ")
			if err != nil {
				return err
			}
			fmt.Fprintln(cmd.OutOrStdout(), string(data))
			return nil
		},
	}
	cmd.Flags().StringArrayVar(&changedFiles, "changed-files", nil, "Files to check (omit to scan all)")
	cmd.Flags().StringVar(&specIDs, "spec", "", "Comma-separated spec IDs to run (omit for all enabled)")
	return cmd
}
