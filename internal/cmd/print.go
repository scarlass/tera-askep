package cmd

import "github.com/spf13/cobra"

var PrintOp = PrintOperation{}

var PrintCmd = cobra.Command{
	Use: "print target",
}

type PrintOperation struct{}
