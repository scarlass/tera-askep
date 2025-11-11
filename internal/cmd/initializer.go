package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/scarlass/tera-askep/internal/core"
	"github.com/scarlass/tera-askep/internal/core/logger"
	"github.com/scarlass/tera-askep/internal/core/utils"
	"github.com/scarlass/tera-askep/internal/resource"
	"github.com/spf13/cobra"
)

var InitOp = InitOperation{
	logger: logger.NewLogger("init"),
}
var InitCmd = cobra.Command{
	Use:   "init",
	Long:  "initialize sync configuration file",
	Short: "initialize sync configuration file",
}

func init() {
	InitOp.setup(&InitCmd)
}

type InitOperation struct {
	logger logger.Logger
}

func (io *InitOperation) setup(cmd *cobra.Command) {
	cmd.RunE = io.action
}
func (io *InitOperation) action(cmd *cobra.Command, args []string) error {
	cwd, _ := os.Getwd()
	filename := filepath.Join(cwd, utils.DEFAULT_CONF_FILE)
	inf, err := os.Stat(filepath.Base(filename))

	if err == nil {
		if inf.IsDir() {
			return core.ErrInvalidConfType(filename)
		}
		return os.ErrExist
	} else if !os.IsNotExist(err) {
		return err
	}

	filenameTemplate := fmt.Sprintf("%s.template", utils.DEFAULT_CONF_FILE)
	content, err := resource.Tmpl.Get(filenameTemplate, map[string]any{
		"conf_loc": cwd,
	})

	if err != nil {
		return err
	}

	if err = os.WriteFile(filename, content, os.ModeAppend); err != nil {
		return err
	}

	io.logger.Printf("Wrote to %s", filename)
	io.logger.Printf("%s", content)
	return nil
}
