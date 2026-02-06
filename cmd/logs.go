package cmd

import (
	"fmt"

	"github.com/heysarver/devinfra/internal/compose"
	"github.com/heysarver/devinfra/internal/config"
	"github.com/spf13/cobra"
)

var logsCmd = &cobra.Command{
	Use:   "logs [project]",
	Short: "Tail infrastructure or project logs",
	GroupID: "infra",
	Args:  cobra.MaximumNArgs(1),
	ValidArgsFunction: projectNameCompletion,
	RunE:  runLogs,
}

func init() {
	rootCmd.AddCommand(logsCmd)
}

func runLogs(cmd *cobra.Command, args []string) error {
	ctx := cmd.Context()

	if len(args) == 0 {
		return compose.Logs(ctx)
	}

	name := args[0]
	reg, err := config.LoadRegistry()
	if err != nil {
		return err
	}
	p := reg.Get(name)
	if p == nil {
		return fmt.Errorf("project %q not found in registry", name)
	}

	files := composeFilesForProject(*p)
	return compose.ProjectLogs(ctx, p.Dir, files)
}
