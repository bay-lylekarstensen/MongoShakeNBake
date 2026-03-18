package utils

import (
	"fmt"
	"os"
	"path/filepath"

	nimo "github.com/gugemichael/nimo4go"
	"github.com/joho/godotenv"
)

// LoadConfigWithEnv reads config text, expands environment variables, and loads it into out.
// Supported env patterns are $VAR and ${VAR}.
func LoadConfigWithEnv(path string, out interface{}) error {
	if err := loadRootDotEnv(); err != nil {
		return err
	}

	raw, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read config file failed: %w", err)
	}

	expanded := os.ExpandEnv(string(raw))
	tmp, err := os.CreateTemp("", "mongoshake-expanded-*.conf")
	if err != nil {
		return fmt.Errorf("create temp config file failed: %w", err)
	}
	tmpPath := tmp.Name()
	defer func() {
		_ = os.Remove(tmpPath)
	}()

	if _, err = tmp.WriteString(expanded); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("write temp config file failed: %w", err)
	}

	if _, err = tmp.Seek(0, 0); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("seek temp config file failed: %w", err)
	}

	loader := nimo.NewConfigLoader(tmp)
	loader.SetDateFormat(GolangSecurityTime)

	if err := loader.Load(out); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("parse expanded config failed: %w", err)
	}

	if err := tmp.Close(); err != nil {
		return fmt.Errorf("close temp config file failed: %w", err)
	}

	return nil
}

func loadRootDotEnv() error {
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("get working directory failed: %w", err)
	}

	dotEnvPath := filepath.Join(cwd, ".env")
	if _, err := os.Stat(dotEnvPath); err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("stat .env failed: %w", err)
	}

	if err := godotenv.Load(dotEnvPath); err != nil {
		return fmt.Errorf("load .env failed: %w", err)
	}

	return nil
}