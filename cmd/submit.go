package cmd

import (
	"errors"

	"github.com/spf13/cobra"
)

var submitCmd = &cobra.Command{
	Use:   "submit",
	Short: "Push the stack and open or update PRs",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		return errors.New("not implemented")
	},
}

func init() {
	rootCmd.AddCommand(submitCmd)
}
