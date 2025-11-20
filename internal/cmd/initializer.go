package cmd

import (
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
	Long:  "initialize sync configuration file in current working directory",
	Short: "initialize sync configuration file",
}

func init() {
	InitOp.setup(&InitCmd)
}

type InitOperation struct {
	Env    bool
	logger logger.Logger
}

func (io *InitOperation) setup(cmd *cobra.Command) {
	fl := cmd.Flags()
	fl.BoolVarP(&io.Env, "env", "e", false, "also generate .env file")

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

	content, err := resource.Tmpl.Get("config.template", map[string]any{
		"conf_loc": cwd,
		"env":      io.Env,
	})

	if err != nil {
		return err
	}

	if err = os.WriteFile(filename, content, 0755); err != nil {
		return err
	}

	if io.Env {
		envContent, err := resource.Tmpl.Get("env.template", map[string]any{})
		if err != nil {
			return err
		}

		envFilename := filepath.Join(cwd, ".env")
		if err = os.WriteFile(envFilename, envContent, 0755); err != nil {
			return err
		}
	}

	io.logger.Printf("Wrote to %s", filename)
	io.logger.Printf("%s", content)
	return nil
}
