package cmd

import (
	"errors"

	"github.com/spf13/cobra"
)

var addCmd = &cobra.Command{
	Use:   "add <slug>",
	Short: "Create the next branch in the stack",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return errors.New("not implemented")
	},
}

func init() {
	rootCmd.AddCommand(addCmd)
}
