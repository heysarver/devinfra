package cmd

import (
	"github.com/heysarver/devinfra/internal/doctor"
	"github.com/heysarver/devinfra/internal/ui"
	"github.com/spf13/cobra"
)

var doctorCmd = &cobra.Command{
	Use:     "doctor",
	Short:   "Health check for devinfra",
	Long:    "Verify tools, containers, DNS, certificates, and per-project health.",
	GroupID: "util",
	RunE:    runDoctor,
}

func init() {
	rootCmd.AddCommand(doctorCmd)
}

func runDoctor(cmd *cobra.Command, args []string) error {
	ctx := cmd.Context()
	report := doctor.RunAll(ctx)

	if flagJSON {
		return ui.PrintJSON(report)
	}

	doctor.PrintReport(report)

	if !report.Passed {
		// Exit code 1 for failures
		cmd.SilenceErrors = true
		return nil
	}
	return nil
}
