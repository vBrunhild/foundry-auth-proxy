package proxy_test

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"foundry-auth-proxy/internal/config"
	"foundry-auth-proxy/internal/identity"
	"foundry-auth-proxy/internal/login"
	"foundry-auth-proxy/internal/proxy"
)

var trustedSource = []string{"10.0.0.0/8"}

func fakeFoundry(t *testing.T) *httptest.Server {
	t.Helper()

	mux := http.NewServeMux()

	mux.HandleFunc("GET /join", func(response http.ResponseWriter, request *http.Request) {
		io.WriteString(response, "native join page")
	})

	mux.HandleFunc("POST /join", func(response http.ResponseWriter, request *http.Request) {
		var body struct{ Action, UserID, Password string }
		json.NewDecoder(request.Body).Decode(&body)

		switch {
		case body.UserID == "banned":
			response.WriteHeader(http.StatusUnauthorized)
			io.WriteString(response, "JOIN.ErrorBanned")

		case body.Password != "secret":
			response.WriteHeader(http.StatusUnauthorized)
			io.WriteString(response, "JOIN.ErrorInvalidPassword")

		default:
			http.SetCookie(response, &http.Cookie{Name: "session", Value: "abc123", SameSite: http.SameSiteStrictMode})
			json.NewEncoder(response).Encode(map[string]string{
				"request":  "join",
				"status":   "success",
				"redirect": "/game",
			})
		}
	})

	mux.HandleFunc("GET /game", func(response http.ResponseWriter, request *http.Request) {
		io.WriteString(response, "game page")
	})

	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)

	return server
}

func newHandler(t *testing.T, foundryURL, onUnmapped string, trustedProxies []string) http.Handler {
	t.Helper()

	directory := t.TempDir()
	writePassword := func(name, contents string) string {
		path := filepath.Join(directory, name)
		if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
			t.Fatal(err)
		}

		return path
	}

	cfg := &config.Config{
		FoundryURL:     foundryURL,
		UserHeader:     "Remote-User",
		OnUnmapped:     onUnmapped,
		TrustedProxies: trustedProxies,
		Users: map[string]config.User{
			"alice":  {FoundryUserId: "aliceid", PasswordFile: writePassword("alice", "secret\n")},
			"bob":    {FoundryUserId: "bobid", PasswordFile: writePassword("bob", "wrong")},
			"banned": {FoundryUserId: "banned", PasswordFile: writePassword("banned", "secret")},
		},
	}

	resolver := identity.HeaderResolver{Header: "Remote-User"}
	quietLogger := slog.New(slog.NewTextHandler(io.Discard, nil))

	handler, err := proxy.New(cfg, resolver, login.NewClient(foundryURL), quietLogger)
	if err != nil {
		t.Fatal(err)
	}

	return handler
}

func serve(handler http.Handler, method, path string, headers map[string]string) *httptest.ResponseRecorder {
	request := httptest.NewRequest(method, path, nil)
	request.Host = "example.com"
	request.RemoteAddr = "10.0.0.5:4000"

	for name, value := range headers {
		request.Header.Set(name, value)
	}

	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)

	return recorder
}

func TestAutoLoginSuccess(t *testing.T) {
	handler := newHandler(t, fakeFoundry(t).URL, config.UnmappedFallthrough, trustedSource)

	recorder := serve(handler, "GET", "/join", map[string]string{"Remote-User": "alice"})

	if recorder.Code != http.StatusFound || recorder.Header().Get("Location") != "/game" {
		t.Fatalf("got %d %q", recorder.Code, recorder.Header().Get("Location"))
	}

	if recorder.Header().Get("Set-Cookie") == "" {
		t.Fatal("session cookie not relayed")
	}
}

func TestRejectedPasswordFallsThrough(t *testing.T) {
	handler := newHandler(t, fakeFoundry(t).URL, config.UnmappedFallthrough, trustedSource)

	for _, subject := range []string{"bob", "banned"} {
		recorder := serve(handler, "GET", "/join", map[string]string{"Remote-User": subject})

		if recorder.Code != http.StatusOK || recorder.Body.String() != "native join page" {
			t.Fatalf("%s: got %d %q", subject, recorder.Code, recorder.Body.String())
		}
	}
}

func TestUnmapped(t *testing.T) {
	foundry := fakeFoundry(t)
	headers := map[string]string{"Remote-User": "mallory"}

	fallthroughHandler := newHandler(t, foundry.URL, config.UnmappedFallthrough, trustedSource)

	recorder := serve(fallthroughHandler, "GET", "/join", headers)
	if recorder.Body.String() != "native join page" {
		t.Fatalf("fallthrough: got %d %q", recorder.Code, recorder.Body.String())
	}

	denyHandler := newHandler(t, foundry.URL, config.UnmappedDeny, trustedSource)

	recorder = serve(denyHandler, "GET", "/join", headers)
	if recorder.Code != http.StatusForbidden {
		t.Fatalf("deny: got %d", recorder.Code)
	}
}

func TestNoIdentity(t *testing.T) {
	foundry := fakeFoundry(t)

	fallthroughHandler := newHandler(t, foundry.URL, config.UnmappedFallthrough, trustedSource)

	recorder := serve(fallthroughHandler, "GET", "/join", nil)
	if recorder.Body.String() != "native join page" {
		t.Fatalf("fallthrough: got %d %q", recorder.Code, recorder.Body.String())
	}

	denyHandler := newHandler(t, foundry.URL, config.UnmappedDeny, trustedSource)

	recorder = serve(denyHandler, "GET", "/join", nil)
	if recorder.Code != http.StatusForbidden {
		t.Fatalf("deny: got %d", recorder.Code)
	}
}

func TestLogoutDoesNotRelogin(t *testing.T) {
	handler := newHandler(t, fakeFoundry(t).URL, config.UnmappedFallthrough, trustedSource)

	testCases := []struct {
		name    string
		path    string
		headers map[string]string
	}{
		{"referer game", "/join", map[string]string{"Remote-User": "alice", "Referer": "http://example.com/game"}},
		{"manual", "/join?manual=1", map[string]string{"Remote-User": "alice"}},
		{"subresource", "/join", map[string]string{"Remote-User": "alice", "Sec-Fetch-Dest": "script"}},
	}

	for _, testCase := range testCases {
		recorder := serve(handler, "GET", testCase.path, testCase.headers)

		if recorder.Code != http.StatusOK || recorder.Body.String() != "native join page" {
			t.Fatalf("%s: got %d %q", testCase.name, recorder.Code, recorder.Body.String())
		}
	}
}

func TestUntrustedSourceHeaderIgnored(t *testing.T) {
	handler := newHandler(t, fakeFoundry(t).URL, config.UnmappedFallthrough, []string{"192.168.0.0/16"})

	recorder := serve(handler, "GET", "/join", map[string]string{"Remote-User": "alice"})

	if recorder.Body.String() != "native join page" {
		t.Fatalf("spoofed header honored: %d %q", recorder.Code, recorder.Body.String())
	}
}

func TestOtherRoutesProxied(t *testing.T) {
	handler := newHandler(t, fakeFoundry(t).URL, config.UnmappedFallthrough, trustedSource)

	recorder := serve(handler, "GET", "/game", map[string]string{"Remote-User": "alice"})

	if recorder.Body.String() != "game page" {
		t.Fatalf("got %q", recorder.Body.String())
	}
}
