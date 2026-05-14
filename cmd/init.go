package cmd

import (
	"errors"

	"github.com/spf13/cobra"
)

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize chainrail in the current repo",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		return errors.New("not implemented")
	},
}

func init() {
	rootCmd.AddCommand(initCmd)
}
