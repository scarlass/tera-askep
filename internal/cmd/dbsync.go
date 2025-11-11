package cmd

import (
	"encoding/base64"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"

	"github.com/scarlass/tera-askep/internal/core"
	"github.com/scarlass/tera-askep/internal/core/configs"
	"github.com/scarlass/tera-askep/internal/core/logger"
	"github.com/scarlass/tera-askep/internal/core/utils"
	"github.com/spf13/cobra"
)

var SyncOp = SyncOperation{
	logger: logger.NewLogger("sync"),
}

var SyncCmd = cobra.Command{
	Use:   "sync targets...",
	Long:  "synchronize target project to teramedik master",
	Short: "synchronize target project to teramedik master",
}

func init() {
	SyncOp.setup(&SyncCmd)
}

type SyncOperation struct {
	ConfigFile string
	Dry        bool

	logger  logger.Logger
	cwd     string
	targets []string
	conf    struct {
		Database configs.DatabaseConfig `mapstructure:"database"`
		Ssh      configs.SSHConfig      `mapstructure:"ssh"`
		Target   configs.TargetConfigs  `mapstructure:"target"`
	}
}

func (so *SyncOperation) setup(cmd *cobra.Command) {
	fl := cmd.Flags()
	fl.BoolVarP(&SyncOp.Dry, "dry", "d", false, "execute with sync to the database also print the html output (concated with script & stylesheet)")
	fl.StringVarP(&SyncOp.ConfigFile, "config", "c", "", "configuration file path, also set virtual cwd based on the configuration file directory")

	cmd.PreRunE = so.preAction
	cmd.RunE = so.action
}

func (so *SyncOperation) preAction(cmd *cobra.Command, args []string) error {
	cwd, err := configs.Load(so.logger, so.ConfigFile, &so.conf)
	if err != nil {
		return err
	}

	so.cwd = cwd
	so.conf.Target.Configure(cwd, "forms")
	if err := so.conf.Database.Validate(); err != nil {
		return err
	}

	so.logger.Debug("target configurations", "conf", so.conf.Target)

	if len(so.conf.Target) == 0 {
		return core.ErrNoTargetAvailable
	} else if len(args) == 0 {
		return core.ErrArgNoTargetSpecified
	}

	so.logger.Debug("command arguments", "args", args)

	so.targets = make([]string, 0)
	for _, spec := range args {
		if act, ok := so.conf.Target.Included(spec); !ok {
			return fmt.Errorf("unknown target (%s) in argument(s)", spec)
		} else {
			so.targets = append(so.targets, act)
		}
	}

	return nil
}

func (so *SyncOperation) action(cmd *cobra.Command, args []string) error {
	psql, err := exec.LookPath("psql")
	if err == nil {
		var wg sync.WaitGroup

		so.logger.Printf("using local psql executable\n")
		for _, targetName := range so.targets {
			wg.Go(func() {
				conf := so.conf.Target[targetName]
				content, err := so.concat_target_files(conf)
				if err != nil {
					panic(err)
				}

				if err := so.psql_local_exec(psql, conf, content); err != nil {
					panic(err)
				}
			})
		}

		wg.Wait()
	} else {
		so.logger.Printf("psql executable not found, connecting through ssh...")
		so.logger.Printf("validating ssh connection settings")
		if err := so.conf.Ssh.Validate(); err != nil {
			return err
		}

		return so.psql_remote_exec()
	}
	return nil
}

func (so *SyncOperation) concat_target_files(target configs.TargetConfig) (string, error) {
	content := []string{}

	if !utils.FileExist(target.Html) {
		return "", fmt.Errorf(`
		[%s] properly specify target html in configuration and make sure the file exists:
		    - current html path -> %s (not found)
		`, target.Name, target.Html)
	}

	if utils.FileExist(target.Script) {
		rel, _ := filepath.Rel(so.cwd, target.Script)
		so.logger.Printf("[%s] embedding script into html -> %s", target.Name, rel)

		script, _ := os.ReadFile(target.Script)
		content = append(content,
			"<script>",
			string(script),
			"</script>",
		)
	}
	if utils.FileExist(target.Stylesheet) {
		rel, _ := filepath.Rel(so.cwd, target.Stylesheet)
		so.logger.Printf("[%s] embedding stylesheet into html -> %s", target.Name, rel)

		stylesheet, _ := os.ReadFile(target.Stylesheet)
		content = append(content,
			"<style>",
			string(stylesheet),
			"</style>",
		)
	}

	html, _ := os.ReadFile(target.Html)
	content = append(content, string(html))
	return strings.Join(content, "\n"), nil
}

func (so *SyncOperation) psql_prepare_arguments(alid int, content string) []string {
	b64Content := base64.StdEncoding.EncodeToString([]byte(content))

	sql := `SET search_path TO {{ .schema }};
	UPDATE askep_list
        SET form_data = convert_from(decode('{{ .content }}', 'base64'), 'UTF8')
	WHERE alid = {{ .alid }};`

	sql = utils.Must(core.ReplaceTemplateString(sql,
		map[string]any{
			"alid":    alid,
			"content": b64Content,
			"schema":  so.conf.Database.Schema,
		}))

	return []string{
		"-h", so.conf.Database.Host,
		"-p", strconv.Itoa(so.conf.Database.Port),
		"-U", so.conf.Database.User,
		"-d", so.conf.Database.Database,
		"-c", sql,
	}
}
func (so *SyncOperation) psql_local_exec(psql string, target configs.TargetConfig, content string) error {
	args := so.psql_prepare_arguments(target.Alid, content)
	so.logger.Printf("[%s] execute %q", target.Name, strings.Join(args[0:len(args)-2], " "))

	cmd := exec.Command(psql, args...)
	cmd.Env = append(cmd.Env, "PGPASSWORD="+so.conf.Database.Password)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return err
	}

	so.logger.Printf("[%s] success", target.Name)
	return nil
}
func (so *SyncOperation) psql_remote_exec() error {
	return errors.New("procedure not implemented")
}
