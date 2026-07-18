package cli

import (
	"io"

	"github.com/kindbrave/claude-knowledger/internal/config"
	"github.com/kindbrave/claude-knowledger/internal/service"
	"github.com/spf13/cobra"
)

func NewRootCommand(svc *service.Service) *cobra.Command {
	return NewRootCommandWithAddress(svc, config.DefaultServerAddress)
}

func NewRootCommandWithAddress(svc *service.Service, address string) *cobra.Command {
	return NewRootCommandWithAddressAndMCPRunner(svc, address, func() error { return nil })
}

func NewRootCommandWithAddressAndMCPRunner(svc *service.Service, address string, runMCP MCPRunner) *cobra.Command {
	return NewRootCommandWithAddressAndRunners(svc, address, "dev", runMCP, func(out, errOut io.Writer) error { return nil }, func(out, errOut io.Writer) error { return nil }, func(out, errOut io.Writer) error { return nil }, nil)
}

func NewRootCommandWithAddressAndRunners(svc *service.Service, address string, version string, runMCP MCPRunner, runClaudeInstall ClaudeInstallRunner, runCodexInstall CodexInstallRunner, runOpenCodeInstall OpenCodeInstallRunner, runUpdate UpdateRunner) *cobra.Command {
	cmd := &cobra.Command{Use: "knowledger"}
	cmd.PersistentFlags().StringVar(&scopeFlag, "scope", "", "knowledge base scope: project, global. Defaults to project when running in a project directory, else global.")
	cmd.AddCommand(newVersionCommand(version))
	cmd.AddCommand(newUpdateCommand(version, runClaudeInstall, runUpdate))
	cmd.AddCommand(newSearchCommand(svc))
	cmd.AddCommand(newGetCommand(svc))
	cmd.AddCommand(newListItemsCommand(svc))
	cmd.AddCommand(newAddCommand(svc))
	cmd.AddCommand(newDeleteCommand(svc))
	cmd.AddCommand(newIndexCommand(svc))
	cmd.AddCommand(newListKBsCommand(svc))
	cmd.AddCommand(newCreateKBCommand(svc))
	cmd.AddCommand(newDeleteKBCommand(svc))
	cmd.AddCommand(newServeCommand(svc, address))
	cmd.AddCommand(newMCPCommand(runMCP))
	cmd.AddCommand(newInstallCommand(runClaudeInstall, runCodexInstall, runOpenCodeInstall))
	cmd.AddCommand(newKBGitKnowledgeAddCommand(svc))
	cmd.AddCommand(newKBGitKnowledgePullCommand(svc))
	cmd.AddCommand(newKBGitKnowledgeListCommand(svc))
	return cmd
}
