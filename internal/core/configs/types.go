package configs

import (
	"errors"
	"fmt"
	"log/slog"
	"maps"
	"path/filepath"
	"strconv"
	"strings"
	"sync"

	"github.com/scarlass/tera-askep/internal/core"
)

type SSHConfig struct {
	validated   bool
	validatedMu *sync.Mutex

	Host     string `mapstructure:"host"`
	Port     int    `mapstructure:"port"`
	User     string `mapstructure:"user"`
	Password string `mapstructure:"password"`
}

func (sc *SSHConfig) configure(profileName string) {
	if sc.validatedMu == nil {
		sc.validatedMu = &sync.Mutex{}
	}

	if v, ok := LookupEnv(profileName, "ssh", "host"); ok && sc.Host == "" {
		sc.Host = v
	}
	if v, ok := LookupEnv(profileName, "ssh", "port"); ok && sc.Port == 0 {
		if v != "" {
			port, err := strconv.Atoi(v)
			if err != nil {
				panic(fmt.Errorf("unable to convert from %q to int: %w", v, err))
			}

			sc.Port = port
		}
	}
	if v, ok := LookupEnv(profileName, "ssh", "user"); ok && sc.User == "" {
		sc.User = v
	}
	if v, ok := LookupEnv(profileName, "ssh", "password"); ok && sc.Password == "" {
		sc.Password = v
	}

}
func (sc *SSHConfig) Validate() error {
	if sc.validatedMu != nil {
		sc.validatedMu.Lock()
		defer sc.validatedMu.Unlock()
	}

	if sc.validated {
		return nil
	}

	if sc.Host == "" {
		sc.Host = "192.168.0.11"
	}
	if sc.Port == 0 {
		sc.Port = 22
	}
	if sc.User == "" {
		return errors.New("ssh.user cannot be empty")
	}
	if sc.Password == "" {
		return errors.New("ssh.password cannot be empty")
	}

	sc.validated = true
	return nil
}

type DatabaseConfig struct {
	Host     string `mapstructure:"host" dsn:"host"`
	Port     int    `mapstructure:"port" dsn:"port"`
	User     string `mapstructure:"user" dsn:"user"`
	Password string `mapstructure:"password" dsn:"password"`
	Database string `mapstructure:"database" dsn:"dbname"`
	Schema   string `mapstructure:"schema" dsn:"search_path"`
}

func (dc *DatabaseConfig) configure(profileName string) {

	if v, ok := LookupEnv(profileName, "database", "host"); ok && dc.Host == "" {
		dc.Host = v
	}
	if v, ok := LookupEnv(profileName, "database", "port"); ok && dc.Port == 0 {
		if v != "" {
			port, err := strconv.Atoi(v)
			if err != nil {
				panic(fmt.Errorf("unable to convert from %q to int: %w", v, err))
			}

			dc.Port = port
		}
	}
	if v, ok := LookupEnv(profileName, "database", "user"); ok && dc.User == "" {
		dc.User = v
	}
	if v, ok := LookupEnv(profileName, "database", "password"); ok && dc.Password == "" {
		dc.Password = v
	}
	if v, ok := LookupEnv(profileName, "database", "database"); ok && dc.Database == "" {
		dc.Database = v
	}
	if v, ok := LookupEnv(profileName, "database", "schema"); ok && dc.Schema == "" {
		dc.Schema = v
	}
}
func (dc *DatabaseConfig) Validate() error {
	if dc.Host == "" {
		// dc.Host = "192.168.0.15"
		return errors.New("database.host cannot be empty")
	}
	if dc.Port == 0 {
		// dc.Port = 5432
		return errors.New("database.port cannot be empty")
	}
	if dc.User == "" {
		return errors.New("database.user cannot be empty")
	}
	if dc.Password == "" {
		return errors.New("database.password cannot be empty")
	}
	if dc.Database == "" {
		// dc.Database = "teramedik_master"
		return errors.New("database.database cannot be empty")
	}
	if dc.Schema == "" {
		dc.Schema = "public"
	}
	return nil
}

// type WatchConfig struct {
// 	Delay time.Duration `mapstructure:"delay"`
// }

type (
	ProfileConfigs map[string]ProfileConfig
	ProfileConfig  struct {
		Name     string
		Env      bool           `mapstructure:"env"`
		Ssh      SSHConfig      `mapstructure:"ssh"`
		Database DatabaseConfig `mapstructure:"database"`
	}
)

func (pcs ProfileConfigs) ValidateAndGet(profile string) (*ProfileConfig, error) {
	if act, ok := pcs.Included(profile); ok {
		p := pcs[act]
		p.Ssh.configure(profile)
		p.Database.configure(profile)

		if err := p.Database.Validate(); err != nil {
			return nil, err
		}

		if pcs[act] != p {
			pcs[act] = p
		}
		return &p, nil
	}
	return nil, fmt.Errorf("profile %s not found", profile)
}
func (pcs ProfileConfigs) Included(profile string) (act string, exist bool) {
	for k := range pcs {
		if strings.EqualFold(k, profile) {
			return k, true
		}
	}
	return profile, false
}
func (pcs *ProfileConfigs) Configure() {
	for name, profile := range *pcs {
		profile.Name = name
		(*pcs)[name] = profile
	}
}

type TargetConfigs map[string]TargetConfig

func (tcs TargetConfigs) Keys() []string {
	k := make([]string, 0)
	for name, _ := range tcs {
		k = append(k, name)
	}
	return k
}
func (tcs TargetConfigs) Included(target string) (actual string, exist bool) {
	for k := range maps.Keys(tcs) {
		if strings.EqualFold(target, k) {
			return k, true
		}
	}
	return "", false
}

type TargetConfig struct {
	Name string
	Alid int `mapstructure:"alid"`

	Html       string             `mapstructure:"html"`
	Stylesheet []string           `mapstructure:"stylesheet"`
	Script     []string           `mapstructure:"script"`
	Options    TargetOptionConfig `mapstructure:"options"`
}
type TargetOptionConfig struct {
	ForceSSH bool `mapstructure:"force-ssh"`
}

func (ts *TargetConfigs) Configure(cwd string) {
	for name, conf := range *ts {
		conf.SetPaths(cwd, name)
		(*ts)[name] = conf
	}
}

func (t *TargetConfig) SetPaths(cwd, name string) {
	defaultPath := filepath.Join(cwd, name)
	t.Name = name

	slog.Debug("set html path", "source", t.Html)
	t.Html = t.withFilepath(cwd, t.Html, filepath.Join(defaultPath, "index.html"))

	if len(t.Script) > 0 {
		slog.Debug("set script path(s)", "source", t.Script)
		for i, script := range t.Script {
			t.Script[i] = t.withFilepath(cwd, script, filepath.Join(defaultPath, "index.js"))
		}
	} else {
		t.Script = []string{}
	}

	if len(t.Stylesheet) > 0 {
		slog.Debug("set stylesheet path(s)", "source", t.Stylesheet)
		for i, style := range t.Stylesheet {
			t.Stylesheet[i] = t.withFilepath(cwd, style, filepath.Join(defaultPath, "index.css"))
		}
	} else {
		t.Stylesheet = []string{}
	}
}

func (t *TargetConfig) withFilepath(cwd, source, defaults string) string {
	if source == "" {
		return defaults
	}

	s, err := core.ReplaceTemplateString(source, map[string]any{
		"cwd":    cwd,
		"target": t.Name,
	})

	if err != nil {
		panic(err)
	}

	slog.Debug("replace file path output", "output", s)
	if filepath.IsAbs(s) {
		return s
	}
	return filepath.Join(cwd, s)
}
