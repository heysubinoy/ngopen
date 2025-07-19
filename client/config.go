package client

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

type Config struct {
	AuthKey        string `json:"auth_key,omitempty"`
	Hostname       string `json:"hostname,omitempty"`
	Local          string `json:"local,omitempty"`
	Server         string `json:"server,omitempty"`
	ReconnectDelay string `json:"reconnect_delay,omitempty"`
	PreserveIP     *bool  `json:"preserve_ip,omitempty"`
}

func configFilePath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	configDir := filepath.Join(home, ".ngopen")
	if err := os.MkdirAll(configDir, 0700); err != nil {
		return "", err
	}
	return filepath.Join(configDir, "config.json"), nil
}

func loadConfig() (*Config, error) {
	path, err := configFilePath()
	if err != nil {
		return nil, err
	}
	f, err := os.Open(path)
	if errors.Is(err, os.ErrNotExist) {
		return &Config{}, nil
	} else if err != nil {
		return nil, err
	}
	defer f.Close()
	var cfg Config
	if err := json.NewDecoder(f).Decode(&cfg); err != nil {
		return nil, err
	}
	return &cfg, nil
}

func saveConfig(cfg *Config) error {
	path, err := configFilePath()
	if err != nil {
		return err
	}
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	enc := json.NewEncoder(f)
	enc.SetIndent("", "  ")
	return enc.Encode(cfg)
}

func getConfigValue(key string) (string, error) {
	cfg, err := loadConfig()
	if err != nil {
		return "", err
	}
	switch key {
	case "auth-key":
		return cfg.AuthKey, nil
	case "hostname":
		return cfg.Hostname, nil
	case "local":
		return cfg.Local, nil
	case "server":
		return cfg.Server, nil
	case "reconnect-delay":
		return cfg.ReconnectDelay, nil
	case "preserve-ip":
		if cfg.PreserveIP == nil {
			return "", nil
		}
		if *cfg.PreserveIP {
			return "true", nil
		}
		return "false", nil
	default:
		return "", fmt.Errorf("unknown config key: %s", key)
	}
}

func listConfig() (map[string]string, error) {
	cfg, err := loadConfig()
	if err != nil {
		return nil, err
	}
	m := map[string]string{}
	if cfg.AuthKey != "" {
		m["auth-key"] = cfg.AuthKey
	}
	if cfg.Hostname != "" {
		m["hostname"] = cfg.Hostname
	}
	if cfg.Local != "" {
		m["local"] = cfg.Local
	}
	if cfg.Server != "" {
		m["server"] = cfg.Server
	}
	if cfg.ReconnectDelay != "" {
		m["reconnect-delay"] = cfg.ReconnectDelay
	}
	if cfg.PreserveIP != nil {
		if *cfg.PreserveIP {
			m["preserve-ip"] = "true"
		} else {
			m["preserve-ip"] = "false"
		}
	}
	return m, nil
}
