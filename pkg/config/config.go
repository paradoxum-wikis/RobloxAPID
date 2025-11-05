package config

import (
	"encoding/json"
	"os"
	"time"
)

type Config struct {
	Server           ServerConfig           `json:"server"`
	Wiki             WikiConfig             `json:"wiki"`
	DynamicEndpoints DynamicEndpointsConfig `json:"dynamicEndpoints"`
	OpenCloud        OpenCloudConfig        `json:"openCloud"`
}

type ServerConfig struct {
	ListenAddress         string `json:"listenAddress"`
	CategoryCheckInterval string `json:"categoryCheckInterval"`
	DataRefreshInterval   string `json:"dataRefreshInterval"`
}

type WikiConfig struct {
	APIURL    string `json:"apiUrl"`
	Username  string `json:"username"`
	Password  string `json:"password"`
	Namespace string `json:"namespace"`
}

type DynamicEndpointsConfig struct {
	CategoryPrefix   string            `json:"categoryPrefix"`
	APIMap           map[string]string `json:"apiMap"`
	RefreshIntervals map[string]string `json:"refreshIntervals"`
}

type OpenCloudConfig struct {
	APIKey string `json:"apiKey"`
}

func LoadConfig(path string) (*Config, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	decoder := json.NewDecoder(file)
	config := &Config{}
	err = decoder.Decode(config)
	if err != nil {
		return nil, err
	}

	return config, nil
}

func (c *Config) GetCategoryCheckInterval() (time.Duration, error) {
	return time.ParseDuration(c.Server.CategoryCheckInterval)
}

func (c *Config) GetDataRefreshInterval() (time.Duration, error) {
	return time.ParseDuration(c.Server.DataRefreshInterval)
}

func (c *Config) GetRefreshInterval(endpointType string) (time.Duration, error) {
	if raw, ok := c.DynamicEndpoints.RefreshIntervals[endpointType]; ok && raw != "" {
		return time.ParseDuration(raw)
	}
	return c.GetDataRefreshInterval()
}
