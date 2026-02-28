package cmd

import (
	"github.com/heysarver/devinfra/internal/project"
	"github.com/spf13/cobra"
)

var regenerateCmd = &cobra.Command{
	Use:     "regenerate",
	Short:   "Regenerate overlays, certs, and dynamic configs for all projects",
	Long:    "Rebuild all docker-compose.devinfra.yaml overlays, TLS certificates, and Traefik dynamic configs from current configuration. Projects that were running are restarted; stopped projects stay stopped.",
	GroupID: "project",
	Args:    cobra.NoArgs,
	RunE:    runRegenerate,
}

func init() {
	rootCmd.AddCommand(regenerateCmd)
}

func runRegenerate(cmd *cobra.Command, args []string) error {
	return project.RegenerateAll(cmd.Context())
}
