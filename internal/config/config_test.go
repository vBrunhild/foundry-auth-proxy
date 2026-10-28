package config_test

import (
	"os"
	"path/filepath"
	"testing"

	"foundry-auth-proxy/internal/config"
)

func loadFromString(t *testing.T, body string) (*config.Config, error) {
	t.Helper()

	path := filepath.Join(t.TempDir(), "config.json")
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}

	return config.Load(path)
}

func TestDefaults(t *testing.T) {
	loaded, err := loadFromString(t, `{"foundryURL":"http://127.0.0.1:30000","trustedProxies":["10.0.0.0/8"]}`)
	if err != nil {
		t.Fatal(err)
	}

	if loaded.Listen != ":8080" || loaded.UserHeader != "Remote-User" || loaded.OnUnmapped != config.UnmappedFallthrough {
		t.Fatalf("defaults not applied: %+v", loaded)
	}
}

func TestInvalid(t *testing.T) {
	invalidConfigs := map[string]string{
		"no url":             `{"trustedProxies":["10.0.0.0/8"]}`,
		"bad unmapped":       `{"foundryURL":"http://x:1","trustedProxies":["10.0.0.0/8"],"onUnmapped":"nope"}`,
		"no trusted proxies": `{"foundryURL":"http://x:1"}`,
		"empty trusted":      `{"foundryURL":"http://x:1","trustedProxies":[]}`,
		"bad cidr":           `{"foundryURL":"http://x:1","trustedProxies":["nope"]}`,
		"empty user":         `{"foundryURL":"http://x:1","trustedProxies":["10.0.0.0/8"],"users":{"a":{}}}`,
		"unknown key":        `{"foundryURL":"http://x:1","trustedProxies":["10.0.0.0/8"],"oops":1}`,
	}

	for name, body := range invalidConfigs {
		if _, err := loadFromString(t, body); err == nil {
			t.Errorf("%s: expected error", name)
		}
	}
}

func TestPasswordTrimsNewline(t *testing.T) {
	passwordFile := filepath.Join(t.TempDir(), "password")
	if err := os.WriteFile(passwordFile, []byte("hunter2\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	password, err := config.User{PasswordFile: passwordFile}.Password()
	if err != nil || password != "hunter2" {
		t.Fatalf("got %q %v", password, err)
	}
}
