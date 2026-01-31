package config

import (
	"encoding/json"
	"fmt"
	"os"
	"time"

	"gopkg.in/yaml.v3"
)

type Config struct {
	ClusterName    string              `json:"clusterName" yaml:"clusterName"`
	KubeConfig     string              `json:"kubeConfig" yaml:"kubeConfig"`
	LogLevel       string              `json:"logLevel" yaml:"logLevel"`
	MetricsAddr    string              `json:"metricsAddr" yaml:"metricsAddr"`
	ProbeAddr      string              `json:"probeAddr" yaml:"probeAddr"`
	LeaderElection bool                `json:"leaderElection" yaml:"leaderElection"`
	Integrations   []IntegrationConfig `json:"integrations" yaml:"integrations"`
	Webhook        WebhookConfig       `json:"webhook" yaml:"webhook"`
	Reconcile      ReconcileConfig     `json:"reconcile" yaml:"reconcile"`
}

type IntegrationConfig struct {
	Name    string                 `json:"name" yaml:"name"`
	Type    string                 `json:"type" yaml:"type"`
	Enabled bool                   `json:"enabled" yaml:"enabled"`
	Config  map[string]interface{} `json:"config" yaml:"config"`
}

type WebhookConfig struct {
	Enabled  bool   `json:"enabled" yaml:"enabled"`
	Port     int    `json:"port" yaml:"port"`
	CertDir  string `json:"certDir" yaml:"certDir"`
	CertName string `json:"certName" yaml:"certName"`
	KeyName  string `json:"keyName" yaml:"keyName"`
}

type ReconcileConfig struct {
	Interval     time.Duration `json:"interval" yaml:"interval"`
	RetryCount   int           `json:"retryCount" yaml:"retryCount"`
	RetryBackoff time.Duration `json:"retryBackoff" yaml:"retryBackoff"`
}

func NewDefaultConfig() *Config {
	return &Config{
		ClusterName:    "default",
		LogLevel:       "info",
		MetricsAddr:    ":8080",
		ProbeAddr:      ":8081",
		LeaderElection: false,
		Webhook: WebhookConfig{
			Enabled: false,
			Port:    9443,
			CertDir: "/tmp/k8s-webhook-server/serving-certs",
		},
		Reconcile: ReconcileConfig{
			Interval:     30 * time.Second,
			RetryCount:   3,
			RetryBackoff: 5 * time.Second,
		},
		Integrations: []IntegrationConfig{},
	}
}

func LoadConfig(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	config := NewDefaultConfig()

	if err := yaml.Unmarshal(data, config); err != nil {
		if err := json.Unmarshal(data, config); err != nil {
			return nil, fmt.Errorf("failed to parse config file: %w", err)
		}
	}

	return config, nil
}

func (c *Config) Save(path string) error {
	data, err := yaml.Marshal(c)
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}

	return nil
}

func (c *Config) Validate() error {
	if c.ClusterName == "" {
		return fmt.Errorf("clusterName is required")
	}

	for _, integration := range c.Integrations {
		if integration.Name == "" {
			return fmt.Errorf("integration name is required")
		}
		if integration.Type == "" {
			return fmt.Errorf("integration type is required for %s", integration.Name)
		}
	}

	return nil
}

func (c *Config) GetIntegration(name string) (*IntegrationConfig, bool) {
	for _, integration := range c.Integrations {
		if integration.Name == name {
			return &integration, true
		}
	}
	return nil, false
}

func (c *Config) GetIntegrationsByType(integrationType string) []IntegrationConfig {
	var result []IntegrationConfig
	for _, integration := range c.Integrations {
		if integration.Type == integrationType {
			result = append(result, integration)
		}
	}
	return result
}
