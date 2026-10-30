package proxy

import (
	"context"
	"errors"
	"log/slog"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"

	"foundry-auth-proxy/internal/config"
	"foundry-auth-proxy/internal/identity"
	"foundry-auth-proxy/internal/login"
)

type Joiner interface {
	Join(ctx context.Context, userID, password, cookieHeader string) (*login.Result, error)
}

type Handler struct {
	config          *config.Config
	resolver        identity.Resolver
	joiner          Joiner
	upstream        http.Handler
	trustedNetworks []*net.IPNet
	logger          *slog.Logger
}

func New(
	cfg *config.Config,
	resolver identity.Resolver,
	joiner Joiner,
	logger *slog.Logger,
) (*Handler, error) {
	foundryURL, err := url.Parse(cfg.FoundryURL)
	if err != nil {
		return nil, err
	}

	handler := &Handler{
		config:   cfg,
		resolver: resolver,
		joiner:   joiner,
		upstream: httputil.NewSingleHostReverseProxy(foundryURL),
		logger:   logger,
	}

	for _, cidr := range cfg.TrustedProxies {
		_, network, err := net.ParseCIDR(cidr)
		if err != nil {
			return nil, err
		}

		handler.trustedNetworks = append(handler.trustedNetworks, network)
	}

	return handler, nil
}

func (handler *Handler) ServeHTTP(response http.ResponseWriter, request *http.Request) {
	if !handler.fromTrustedProxy(request) {
		request.Header.Del(handler.config.UserHeader)
	}

	if handler.wantsAutoLogin(request) && handler.autoLogin(response, request) {
		return
	}

	handler.upstream.ServeHTTP(response, request)
}

func (handler *Handler) fromTrustedProxy(request *http.Request) bool {
	host, _, err := net.SplitHostPort(request.RemoteAddr)
	if err != nil {
		return false
	}

	sourceIP := net.ParseIP(host)

	for _, network := range handler.trustedNetworks {
		if network.Contains(sourceIP) {
			return true
		}
	}

	return false
}

func (handler *Handler) wantsAutoLogin(request *http.Request) bool {
	if request.Method != http.MethodGet || request.URL.Path != "/join" {
		return false
	}

	fetchDestination := request.Header.Get("Sec-Fetch-Dest")
	if fetchDestination != "" && fetchDestination != "document" && fetchDestination != "empty" {
		return false
	}

	if request.URL.Query().Has("manual") {
		return false
	}

	return !handler.cameFromGame(request)
}

func (handler *Handler) cameFromGame(request *http.Request) bool {
	referrer, err := url.Parse(request.Referer())
	if err != nil || referrer.Host != request.Host {
		return false
	}

	return strings.HasSuffix(strings.TrimRight(referrer.Path, "/"), "/game")
}

func (handler *Handler) autoLogin(response http.ResponseWriter, request *http.Request) bool {
	subject, found := handler.resolver.Subject(request)
	if !found {
		return handler.denyUnmapped(response, "no identity provided")
	}

	users, err := config.LoadUsers(handler.config.UsersFile)
	if err != nil {
		handler.logger.Error("load users", "err", err)
		http.Error(response, "login misconfigured", http.StatusInternalServerError)

		return true
	}

	user, mapped := users[subject]
	if !mapped {
		handler.logger.Warn("no foundry user mapped", "subject", subject)

		return handler.denyUnmapped(response, "no Foundry user is mapped to this account")
	}

	result, err := handler.joiner.Join(request.Context(), user.FoundryUserId, user.Password, request.Header.Get("Cookie"))
	if errors.Is(err, login.ErrRejected) {
		handler.logger.Warn("foundry rejected login", "subject", subject, "err", err)
		return false
	}

	if err != nil {
		handler.logger.Error("foundry login", "subject", subject, "err", err)
		http.Error(response, "foundry unavailable", http.StatusBadGateway)

		return true
	}

	for _, setCookie := range result.SetCookies {
		response.Header().Add("Set-Cookie", setCookie)
	}

	handler.logger.Info("logged in", "subject", subject, "foundryUser", user.FoundryUserId)
	http.Redirect(response, request, redirectTarget(result.Redirect), http.StatusFound)

	return true
}

func (handler *Handler) denyUnmapped(response http.ResponseWriter, message string) bool {
	if handler.config.OnUnmapped != config.UnmappedDeny {
		return false
	}

	http.Error(response, message, http.StatusForbidden)

	return true
}

func redirectTarget(foundryRedirect string) string {
	isLocalPath := strings.HasPrefix(foundryRedirect, "/") && !strings.HasPrefix(foundryRedirect, "//")
	if isLocalPath {
		return foundryRedirect
	}

	return "/game"
}
