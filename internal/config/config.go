package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/url"
	"os"
)

const (
	UnmappedFallthrough = "fallthrough"
	UnmappedDeny        = "deny"
)

type User struct {
	FoundryUserId string `json:"foundryUserId"`
	Password      string `json:"password"`
}

type Config struct {
	FoundryURL     string   `json:"foundryURL"`
	Listen         string   `json:"listen"`
	UserHeader     string   `json:"userHeader"`
	OnUnmapped     string   `json:"onUnmapped"`
	TrustedProxies []string `json:"trustedProxies"`
	UsersFile      string   `json:"usersFile"`
}

func Load(path string) (*Config, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var config Config

	decoder := json.NewDecoder(file)
	decoder.DisallowUnknownFields()

	if err := decoder.Decode(&config); err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}

	if err := config.applyDefaults(); err != nil {
		return nil, err
	}

	return &config, nil
}

func (config *Config) applyDefaults() error {
	if config.Listen == "" {
		config.Listen = ":8080"
	}

	if config.UserHeader == "" {
		config.UserHeader = "Remote-User"
	}

	if config.OnUnmapped == "" {
		config.OnUnmapped = UnmappedFallthrough
	}

	return config.validate()
}

func (config *Config) validate() error {
	foundryURL, err := url.Parse(config.FoundryURL)
	if err != nil || foundryURL.Scheme == "" || foundryURL.Host == "" {
		return fmt.Errorf("foundryURL %q must be an absolute URL", config.FoundryURL)
	}

	if config.OnUnmapped != UnmappedFallthrough && config.OnUnmapped != UnmappedDeny {
		return fmt.Errorf("onUnmapped must be %q or %q", UnmappedFallthrough, UnmappedDeny)
	}

	if len(config.TrustedProxies) == 0 {
		return errors.New("trustedProxies must list at least one CIDR")
	}

	for _, cidr := range config.TrustedProxies {
		if _, _, err := net.ParseCIDR(cidr); err != nil {
			return fmt.Errorf("trustedProxies: %w", err)
		}
	}

	if config.UsersFile == "" {
		return errors.New("usersFile is required")
	}

	return nil
}

func LoadUsers(path string) (map[string]User, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var users map[string]User

	decoder := json.NewDecoder(file)
	decoder.DisallowUnknownFields()

	if err := decoder.Decode(&users); err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}

	for name, user := range users {
		if user.FoundryUserId == "" || user.Password == "" {
			return nil, fmt.Errorf("user %q needs foundryUserId and password", name)
		}
	}

	return users, nil
}
