package config

import (
	"encoding/json"
	"os"
	"path/filepath"
)

type Config struct {
	PortHistory    map[string]map[string]int `json:"port_history"`
	BastionHistory map[string]string         `json:"bastion_history"`
	DownloadDir    string                    `json:"download_dir"`
	EC2CwdHistory  map[string]string         `json:"ec2_cwd_history"`
}

func configPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".ssm_session_manager", "config.json")
}

func Load() (*Config, error) {
	cfg := &Config{
		PortHistory:    map[string]map[string]int{"RDS": {}, "ElastiCache": {}},
		BastionHistory: map[string]string{},
		EC2CwdHistory:  map[string]string{},
	}

	data, err := os.ReadFile(configPath())
	if err != nil {
		if os.IsNotExist(err) {
			return cfg, nil
		}
		return nil, err
	}

	if err := json.Unmarshal(data, cfg); err != nil {
		return nil, err
	}

	if cfg.PortHistory == nil {
		cfg.PortHistory = map[string]map[string]int{"RDS": {}, "ElastiCache": {}}
	}
	if cfg.PortHistory["RDS"] == nil {
		cfg.PortHistory["RDS"] = map[string]int{}
	}
	if cfg.PortHistory["ElastiCache"] == nil {
		cfg.PortHistory["ElastiCache"] = map[string]int{}
	}
	if cfg.BastionHistory == nil {
		cfg.BastionHistory = map[string]string{}
	}
	if cfg.EC2CwdHistory == nil {
		cfg.EC2CwdHistory = map[string]string{}
	}
	if cfg.DownloadDir == "" {
		home, _ := os.UserHomeDir()
		cfg.DownloadDir = filepath.Join(home, "Downloads", "paws")
	}

	return cfg, nil
}

func (c *Config) Save() error {
	dir := filepath.Dir(configPath())
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(configPath(), data, 0644)
}

func (c *Config) GetSavedPort(instanceType, instanceID string) int {
	if ports, ok := c.PortHistory[instanceType]; ok {
		return ports[instanceID]
	}
	return 0
}

func (c *Config) SetPort(instanceType, instanceID string, port int) {
	if c.PortHistory[instanceType] == nil {
		c.PortHistory[instanceType] = map[string]int{}
	}
	c.PortHistory[instanceType][instanceID] = port
}

func (c *Config) GetSavedBastion(instanceID string) string {
	return c.BastionHistory[instanceID]
}

func (c *Config) SetBastion(instanceID, bastionID string) {
	c.BastionHistory[instanceID] = bastionID
}

func (c *Config) GetEC2Cwd(instanceID string) string {
	return c.EC2CwdHistory[instanceID]
}

func (c *Config) SetEC2Cwd(instanceID, cwd string) {
	if c.EC2CwdHistory == nil {
		c.EC2CwdHistory = map[string]string{}
	}
	c.EC2CwdHistory[instanceID] = cwd
}

func (c *Config) GetDownloadDir() string {
	if c.DownloadDir == "" {
		home, _ := os.UserHomeDir()
		return filepath.Join(home, "Downloads", "paws")
	}
	return c.DownloadDir
}
