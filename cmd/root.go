package cmd

import "github.com/spf13/cobra"

var rootCmd = &cobra.Command{
	Use:   "chainrail",
	Short: "Stacked-PR CLI for GitHub",
	Long:  "chainrail manages chains of dependent pull requests on GitHub. Wraps the gh CLI.",
}

func Execute() error {
	return rootCmd.Execute()
}
