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
	Roblox           RobloxConfig           `json:"roblox"`
	LuaMessages      LuaMessagesConfig      `json:"luaMessages"`
}

type LuaMessagesConfig struct {
	QueueNote         string `json:"queueNote"`
	FieldPathNotFound string `json:"fieldPathNotFound"`
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

type RobloxConfig struct {
	Cookie string `json:"cookie"`
}

func LoadConfig(path string) (*Config, error) {
	file, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	expanded := os.ExpandEnv(string(file))
	config := &Config{}
	err = json.Unmarshal([]byte(expanded), config)
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
