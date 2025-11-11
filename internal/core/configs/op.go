package configs

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/scarlass/tera-askep/internal/core/utils"
	"github.com/spf13/viper"
)

func Lookup(cwd string, filename string) (string, error) {
	dir := cwd
	for {
		var parent string

		file := filepath.Join(dir, filename)
		if inf, serr := os.Stat(file); serr == nil {
			if inf.IsDir() {
				parent = filepath.Dir(dir)
			} else {
				return file, nil
			}
		} else {
			parent = filepath.Dir(dir)
		}

		if parent == dir {
			break
		}
		dir = parent
	}

	return "", os.ErrNotExist
}

func Load(config string, target any) (cwd string, err error) {
	utils.MustPointer(target)

	if config == "" {
		cwd, _ = os.Getwd()
		config, err = Lookup(cwd, utils.DEFAULT_CONF_FILE)
		if err != nil {
			return
		}
	} else {
		cwd = filepath.Dir(config)

		info, err := os.Stat(config)
		if err != nil {
			return cwd, err
		} else if info.IsDir() {
			return cwd, errors.New("target path is a directory")
		}
	}

	// if logger != nil {
	// 	logger.Printf("using configuration found at %s", config)
	// }

	file, err := os.ReadFile(config)
	if err != nil {
		return cwd, fmt.Errorf("unable to read file: %w", err)
	}

	reader := bytes.NewBuffer(file)

	viper.SetConfigType("yaml")
	if err := viper.ReadConfig(reader); err != nil {
		return cwd, fmt.Errorf("unable to apply configuration: %w", err)
	}

	if err := viper.Unmarshal(target); err != nil {
		return cwd, fmt.Errorf("unmarshal failed: %w", err)
	}

	return
}
