package configs

import (
	"errors"
	"fmt"
	"log/slog"
	"maps"
	"path/filepath"
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

func (sc *SSHConfig) configure() {
	if sc.validatedMu == nil {
		sc.validatedMu = &sync.Mutex{}
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
	Host     string `mapstructure:"host"`
	Port     int    `mapstructure:"port"`
	User     string `mapstructure:"user"`
	Password string `mapstructure:"password"`
	Database string `mapstructure:"database"`
	Schema   string `mapstructure:"schema"`
}

func (dc *DatabaseConfig) configure() {}
func (dc *DatabaseConfig) Validate() error {
	if dc.Host == "" {
		dc.Host = "192.168.0.15"
	}
	if dc.Port == 0 {
		dc.Port = 5432
	}
	if dc.User == "" {
		return errors.New("database.user cannot be empty")
	}
	if dc.Password == "" {
		return errors.New("database.password cannot be empty")
	}
	if dc.Database == "" {
		dc.Database = "teramedik_master"
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
		Ssh      SSHConfig      `mapstructure:"ssh"`
		Database DatabaseConfig `mapstructure:"database"`
	}
)

func (pcs ProfileConfigs) ValidateAndGet(profile string) (*ProfileConfig, error) {
	if act, ok := pcs.Included(profile); ok {
		p := pcs[act]
		p.Ssh.configure()
		p.Database.configure()

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

	// Path       string
	Html       string             `mapstructure:"html"`
	Stylesheet string             `mapstructure:"stylesheet"`
	Script     string             `mapstructure:"script"`
	Options    TargetOptionConfig `mapstructure:"options"`
}
type TargetOptionConfig struct {
	ForceSSH bool `mapstructure:"force-ssh"`
}

func (ts *TargetConfigs) Configure(cwd string) {
	delete(*ts, "*")

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

	slog.Debug("set script path", "source", t.Script)
	t.Script = t.withFilepath(cwd, t.Script, filepath.Join(defaultPath, "index.js"))

	slog.Debug("set stylesheet path", "source", t.Stylesheet)
	t.Stylesheet = t.withFilepath(cwd, t.Stylesheet, filepath.Join(defaultPath, "index.css"))
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
