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
	Profile    string

	logger logger.Logger
	cwd    string

	confTargets []configs.TargetConfig
	confProfile *configs.ProfileConfig
	conf        struct {
		Profiles configs.ProfileConfigs `mapstructure:"profile"`
		Targets  configs.TargetConfigs  `mapstructure:"target"`
	}
}

func (so *SyncOperation) setup(cmd *cobra.Command) {
	fl := cmd.Flags()
	fl.BoolVarP(&SyncOp.Dry, "dry", "d", false, "run without sync to the database and print the html output (concated with script & stylesheet)")
	fl.StringVarP(&SyncOp.ConfigFile, "config", "c", "", "configuration file path, also set virtual cwd based on the configuration file directory")
	fl.StringVarP(&SyncOp.Profile, "profile", "p", "default", "specify connection profile to use in configuration file")

	cmd.PreRunE = so.preAction
	cmd.RunE = so.action
}

func (so *SyncOperation) preAction(cmd *cobra.Command, args []string) error {
	cwd, err := configs.Load(so.ConfigFile, &so.conf)
	if err != nil {
		return err
	}

	so.cwd = cwd
	so.logger.SetDry(so.Dry)
	so.conf.Profiles.Configure()
	so.conf.Targets.Configure(cwd)

	if so.Profile == "" {
		return core.ErrEmptyProfile
	}

	if so.confProfile, err = so.conf.Profiles.ValidateAndGet(so.Profile); err != nil {
		return err
	} else {
		so.logger.Printf(`using "%s" profile`, so.confProfile.Name)
	}

	if len(so.conf.Targets) == 0 {
		return core.ErrNoTargetAvailable
	} else if len(args) == 0 {
		return core.ErrArgNoTargetSpecified
	}

	so.logger.Debug("available targets", "targets", so.conf.Targets.Keys())
	so.logger.Debug("command arguments", "args", args)

	so.confTargets = make([]configs.TargetConfig, 0)
	for _, spec := range args {
		if act, ok := so.conf.Targets.Included(spec); !ok {
			return fmt.Errorf("unknown target (%s) in argument(s)", spec)
		} else {
			so.confTargets = append(so.confTargets, so.conf.Targets[act])
		}
	}

	return nil
}

func (so *SyncOperation) action(cmd *cobra.Command, args []string) error {
	if so.Dry {
		return so.action_dry()
	}

	return so.action_main()
}
func (so *SyncOperation) action_dry() error {
	for _, conf := range so.confTargets {
		content, err := so.concat_target_files(conf)

		if err != nil {
			return err
		}

		fmt.Println(content)
	}
	return nil
}
func (so *SyncOperation) action_main() error {
	psql, err := exec.LookPath("psql")
	if err == nil {
		var wg sync.WaitGroup

		so.logger.Printf("using local psql executable\n")
		for _, conf := range so.confTargets {
			wg.Go(func() {
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
		if err := so.confProfile.Ssh.Validate(); err != nil {
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
			"schema":  so.confProfile.Database.Schema,
		}))

	return []string{
		"-h", so.confProfile.Database.Host,
		"-p", strconv.Itoa(so.confProfile.Database.Port),
		"-U", so.confProfile.Database.User,
		"-d", so.confProfile.Database.Database,
		"-c", sql,
	}
}
func (so *SyncOperation) psql_local_exec(psql string, target configs.TargetConfig, content string) error {
	args := so.psql_prepare_arguments(target.Alid, content)
	so.logger.Printf("[%s] execute %q", target.Name, strings.Join(args[0:len(args)-2], " "))

	cmd := exec.Command(psql, args...)
	cmd.Env = append(cmd.Env, "PGPASSWORD="+so.confProfile.Database.Password)
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
