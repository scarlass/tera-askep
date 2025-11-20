package main

import (
	"github.com/scarlass/tera-askep/internal/cmd"
	"github.com/spf13/cobra"
)

func main() {
	root := cobra.Command{
		Use: "tera-askep",
		CompletionOptions: cobra.CompletionOptions{
			HiddenDefaultCmd:  true,
			DisableDefaultCmd: true,
		},
	}

	root.AddCommand(&cmd.InitCmd, &cmd.SyncCmd)
	root.Execute()
}
