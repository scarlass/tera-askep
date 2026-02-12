package cmd

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"sync"

	"github.com/scarlass/tera-askep/internal/cmd/dbsync"
	"github.com/scarlass/tera-askep/internal/core"
	"github.com/scarlass/tera-askep/internal/core/configs"
	"github.com/scarlass/tera-askep/internal/core/db"
	"github.com/scarlass/tera-askep/internal/core/logger"
	"github.com/scarlass/tera-askep/internal/core/ssh"
	"github.com/scarlass/tera-askep/internal/core/utils"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var SyncOp = SyncOperation{
	logger: logger.NewLogger("sync"),
}

var SyncCmd = cobra.Command{
	Use:   "sync targets...",
	Short: "synchronize target project to askep_list table",
	Long:  "synchronize target project to askep_list table and change form_data column",
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

	targetsConf []configs.TargetConfig

	profileConf *configs.ProfileConfig
	profileDb   *db.Database

	conf struct {
		Profiles configs.ProfileConfigs `mapstructure:"profile"`
		Targets  configs.TargetConfigs  `mapstructure:"target"`
	}

	ssh   *ssh.SSHClient
	sshMu *sync.Mutex
}

func (so *SyncOperation) setup(cmd *cobra.Command) {
	so.sshMu = &sync.Mutex{}

	fl := cmd.Flags()
	fl.StringVarP(&SyncOp.ConfigFile, "config", "c", "", dbsync.ConfigFlagDesc)
	fl.BoolVarP(&SyncOp.Dry, "dry", "d", false, dbsync.DryFlagDesc)
	fl.StringVarP(&SyncOp.Profile, "profile", "p", "default", dbsync.ProfileFlagDesc)

	cmd.PreRunE = so.preAction
	cmd.RunE = so.action
	cmd.PostRunE = so.postAction
}

func (so *SyncOperation) preAction(cmd *cobra.Command, args []string) error {
	// dbsync.LoadEnv($so)
	cwd, err := configs.FindAndLoad(so.ConfigFile, &so.conf)
	if err != nil {
		return err
	}

	jsoned, _ := json.MarshalIndent(viper.AllSettings(), "", "    ")
	so.logger.Debugf("loaded config: %s", string(jsoned))

	so.cwd = cwd
	so.logger.SetDry(so.Dry)

	delete(so.conf.Targets, "*")
	so.conf.Targets.Configure(cwd)
	so.conf.Profiles.Configure()

	if so.Profile == "" {
		return core.ErrEmptyProfile
	}

	if so.profileConf, err = so.conf.Profiles.ValidateAndGet(so.Profile); err != nil {
		return err
	} else {
		so.logger.Printf(`using "%s" profile`, so.profileConf.Name)
	}

	if len(so.conf.Targets) == 0 {
		return core.ErrNoTargetAvailable
	} else if len(args) == 0 {
		return core.ErrArgNoTargetSpecified
	}

	so.logger.Debug("available targets", "targets", so.conf.Targets.Keys())
	so.logger.Debug("command arguments", "args", args)

	so.targetsConf = make([]configs.TargetConfig, 0)
	for _, spec := range args {
		if act, ok := so.conf.Targets.Included(spec); !ok {
			return fmt.Errorf("unknown target (%s) in argument(s)", spec)
		} else {
			so.targetsConf = append(so.targetsConf, so.conf.Targets[act])
		}
	}

	so.profileDb, err = db.New(cmd.Context(), so.logger, so.profileConf.Database)
	if err != nil {
		return fmt.Errorf("create db connection (profile - %s): %w", so.profileConf.Name, err)
	}

	// for _, prof := range so.conf.Profiles {

	// 	so.profileDb[prof.Name] = db
	// }

	return nil
}
func (so *SyncOperation) postAction(cmd *cobra.Command, args []string) error {
	if so.profileDb != nil {
		so.profileDb.Close()
	}
	return nil
}

func (so *SyncOperation) action(cmd *cobra.Command, args []string) error {
	if so.Dry {
		return so.action_dry()
	}

	jsoned, _ := json.MarshalIndent(so.conf, "", "    ")
	so.logger.Debugf("serialized config: %s", string(jsoned))

	return so.action_main(cmd.Context())
}
func (so *SyncOperation) action_dry() error {
	for _, conf := range so.targetsConf {
		content, err := so.concat_target_files(conf)

		if err != nil {
			return err
		}

		fmt.Println(content)
	}
	return nil
}
func (so *SyncOperation) action_main(ctx context.Context) error {
	// psql, err := exec.LookPath("psql")
	// psqlExist := err == nil

	// if psqlExist {
	// 	so.logger.Printf("using local psql executable\n")
	// }

	// defer so.ssh.Close()

	var wg sync.WaitGroup
	for _, conf := range so.targetsConf {
		content, err := so.concat_target_files(conf)
		if err != nil {
			return err
		}

		b64Content := base64.StdEncoding.EncodeToString([]byte(content))
		if err := so.profileDb.UpdateAskepList(ctx, conf, b64Content); err != nil {
			return err
		}
		// so.profileDb.UpdateAskepList(so.)
		// wg.Go(func() {
		// 	content, err := so.concat_target_files(conf)
		// 	if err != nil {
		// 		panic(err)
		// 	}

		// 	so.logger.Debug("force ssh enabled ?",
		// 		"target", conf.Name,
		// 		"enabled", conf.Options.ForceSSH)

		// 	if conf.Options.ForceSSH || !psqlExist {
		// 		utils.Must(1, so.psql_remote_exec(conf, content))
		// 	} else {
		// 		utils.Must(1, so.psql_local_exec(psql, conf, content))
		// 	}

		so.logger.Printf("[%s] success", conf.Name)
		// })

	}
	wg.Wait()

	return nil
}

func (so *SyncOperation) concat_target_files(target configs.TargetConfig) (string, error) {
	scripts := []string{}
	stylesheets := []string{}
	// content := []string{}

	if !utils.FileExist(target.Html) {
		return "", fmt.Errorf(`
		[%s] properly specify target html in configuration and make sure the file exists:
		    - current html path -> %s (not found)
		`, target.Name, target.Html)
	}

	so.logger.Debug("total stylesheet(s)", "count", len(target.Stylesheet))
	if len(target.Stylesheet) > 0 {
		for i, style := range target.Stylesheet {
			exist := utils.FileExist(style)
			so.logger.Debugf("%d. %s exist ? %s", i+1, style, exist)

			if !exist {
				continue
			}

			rel, _ := filepath.Rel(so.cwd, style)
			so.logger.Printf("[%s] embedding stylesheet into html -> %s", target.Name, rel)

			section, _ := os.ReadFile(style)
			section_str := "<!-- " + style + " -->\n<style>\n" + string(section) + "\n</style>"

			stylesheets = append(stylesheets, section_str)
			// content = append(content, section_str)
		}
	}

	// html, _ := os.ReadFile(target.Html)
	// content = append(content, string(html))

	so.logger.Debug("total script(s)", "count", len(target.Script))
	if len(target.Script) > 0 {
		for _, script := range target.Script {
			if !utils.FileExist(script) {
				continue
			}

			rel, _ := filepath.Rel(so.cwd, script)
			so.logger.Printf("[%s] embedding script into html -> %s", target.Name, rel)

			section, _ := os.ReadFile(script)
			section_str := "<!-- " + script + " -->\n<script>\n" + string(section) + "\n</script>"

			scripts = append(scripts, section_str)
		}
	}

	html, _ := os.ReadFile(target.Html)

	ss_rgx := regexp.MustCompile(`\{\{\s*\.Stylesheet\s*\}\}`)
	ss_indexes := ss_rgx.FindSubmatchIndex(html)
	if len(ss_indexes) > 0 {
		html = slices.Concat(
			html[:ss_indexes[0]],
			[]byte(strings.Join(stylesheets, "\n")),
			html[ss_indexes[1]:],
		)
	} else {
		html = slices.Concat(
			[]byte(strings.Join(stylesheets, "\n")),
			[]byte{'\n', '\n'},
			html,
		)
	}

	scr_rgx := regexp.MustCompile(`\{\{\s*\.Script\s*\}\}`)
	scr_indexes := scr_rgx.FindSubmatchIndex(html)
	if len(scr_indexes) > 0 {
		html = slices.Concat(
			html[:scr_indexes[0]],
			[]byte(strings.Join(scripts, "\n")),
			html[scr_indexes[1]:],
		)
	} else {
		html = slices.Concat(
			html,
			[]byte{'\n', '\n'},
			[]byte(strings.Join(scripts, "\n")),
		)
	}

	return string(html), nil
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
			"schema":  so.profileConf.Database.Schema,
		}))

	return []string{
		"-h", so.profileConf.Database.Host,
		"-p", strconv.Itoa(so.profileConf.Database.Port),
		"-U", so.profileConf.Database.User,
		"-d", so.profileConf.Database.Database,
		"-c", sql,
	}
}
func (so *SyncOperation) psql_local_exec(psql string, target configs.TargetConfig, content string) error {
	args := so.psql_prepare_arguments(target.Alid, content)
	so.logger.Printf("[%s] psql prepared argument(s) %q", target.Name, strings.Join(args[0:len(args)-2], " "))

	cmd := exec.Command(psql, args...)
	cmd.Env = append(cmd.Env, "PGPASSWORD="+so.profileConf.Database.Password)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return err
	}

	return nil
}
func (so *SyncOperation) psql_remote_exec(target configs.TargetConfig, content string) (err error) {
	so.sshMu.Lock()
	if so.ssh == nil {
		if so.ssh, err = ssh.New(so.profileConf.Ssh); err != nil {
			so.sshMu.Unlock()
			panic(err)
		}

		so.logger.Printf("[%s] ssh client connected", target.Name)
	}
	so.sshMu.Unlock()

	args := so.psql_prepare_arguments(target.Alid, content)
	last := len(args) - 1

	so.logger.Printf("[%s] psql prepared argument(s) %q", target.Name, strings.Join(args[0:len(args)-2], " "))
	args[last] = fmt.Sprintf(`"%s"`, args[last])

	if err := so.ssh.Exec(fmt.Sprintf("PGPASSWORD=%s; psql", so.profileConf.Database.Password), args...); err != nil {
		return err
	}

	return nil
}
