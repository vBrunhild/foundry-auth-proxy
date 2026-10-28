package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/url"
	"os"
	"strings"
)

const (
	UnmappedFallthrough = "fallthrough"
	UnmappedDeny        = "deny"
)

type User struct {
	FoundryUserId string `json:"foundryUserId"`
	PasswordFile  string `json:"passwordFile"`
}

type Config struct {
	FoundryURL     string          `json:"foundryURL"`
	Listen         string          `json:"listen"`
	UserHeader     string          `json:"userHeader"`
	OnUnmapped     string          `json:"onUnmapped"`
	TrustedProxies []string        `json:"trustedProxies"`
	Users          map[string]User `json:"users"`
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

	for name, user := range config.Users {
		if user.FoundryUserId == "" || user.PasswordFile == "" {
			return fmt.Errorf("user %q needs foundryUserId and passwordFile", name)
		}
	}

	return nil
}

func (user User) Password() (string, error) {
	contents, err := os.ReadFile(user.PasswordFile)
	if err != nil {
		return "", err
	}

	password := strings.TrimRight(string(contents), "\r\n")
	if password == "" {
		return "", errors.New("password file is empty: " + user.PasswordFile)
	}

	return password, nil
}
