package cmd

import (
	"fmt"

	"github.com/heysarver/devinfra/internal/ui"
	"github.com/heysarver/devinfra/internal/version"
	"github.com/spf13/cobra"
)

var versionCmd = &cobra.Command{
	Use:     "version",
	Short:   "Print version information",
	GroupID: "util",
	RunE: func(cmd *cobra.Command, args []string) error {
		info := version.Get()
		if flagJSON {
			return ui.PrintJSON(info)
		}
		fmt.Println(info.String())
		return nil
	},
}

func init() {
	rootCmd.AddCommand(versionCmd)
}
