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
	loaded, err := loadFromString(t, `{"foundryURL":"http://127.0.0.1:30000","trustedProxies":["10.0.0.0/8"],"usersFile":"/u.json"}`)
	if err != nil {
		t.Fatal(err)
	}

	if loaded.Listen != ":8080" || loaded.UserHeader != "Remote-User" || loaded.OnUnmapped != config.UnmappedFallthrough {
		t.Fatalf("defaults not applied: %+v", loaded)
	}
}

func TestInvalid(t *testing.T) {
	invalidConfigs := map[string]string{
		"no url":             `{"trustedProxies":["10.0.0.0/8"],"usersFile":"/u.json"}`,
		"bad unmapped":       `{"foundryURL":"http://x:1","trustedProxies":["10.0.0.0/8"],"usersFile":"/u.json","onUnmapped":"nope"}`,
		"no trusted proxies": `{"foundryURL":"http://x:1"}`,
		"empty trusted":      `{"foundryURL":"http://x:1","trustedProxies":[]}`,
		"bad cidr":           `{"foundryURL":"http://x:1","trustedProxies":["nope"]}`,
		"no users file":      `{"foundryURL":"http://x:1","trustedProxies":["10.0.0.0/8"]}`,
		"unknown key":        `{"foundryURL":"http://x:1","trustedProxies":["10.0.0.0/8"],"usersFile":"/u.json","oops":1}`,
	}

	for name, body := range invalidConfigs {
		if _, err := loadFromString(t, body); err == nil {
			t.Errorf("%s: expected error", name)
		}
	}
}

func TestLoadUsers(t *testing.T) {
	path := filepath.Join(t.TempDir(), "users.json")
	body := `{"alice":{"foundryUserId":"AbC","password":"hunter2"}}`
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}

	users, err := config.LoadUsers(path)
	if err != nil {
		t.Fatal(err)
	}

	if users["alice"].FoundryUserId != "AbC" || users["alice"].Password != "hunter2" {
		t.Fatalf("got %+v", users)
	}
}

func TestLoadUsersInvalid(t *testing.T) {
	invalidUsers := map[string]string{
		"missing password": `{"alice":{"foundryUserId":"AbC"}}`,
		"missing id":       `{"alice":{"password":"hunter2"}}`,
		"unknown key":      `{"alice":{"foundryUserId":"AbC","password":"x","oops":1}}`,
		"not an object":    `[]`,
	}

	for name, body := range invalidUsers {
		path := filepath.Join(t.TempDir(), "users.json")
		if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}

		if _, err := config.LoadUsers(path); err == nil {
			t.Errorf("%s: expected error", name)
		}
	}
}

func TestLoadUsersMissingFile(t *testing.T) {
	if _, err := config.LoadUsers(filepath.Join(t.TempDir(), "absent.json")); err == nil {
		t.Fatal("expected error")
	}
}
