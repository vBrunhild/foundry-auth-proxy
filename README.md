# foundry-auth-proxy

Reverse proxy for Foundry VTT that skips the native `/join` screen. It trusts an identity header set by an upstream auth proxy, maps that identity to a Foundry user, and replays Foundry's `POST /join` on the user's behalf, relaying the session cookie.

Foundry sessions are in-memory and cannot be created externally, so every mapped Foundry user needs a real access key, stored in a file readable by the proxy.

## Config

```json
{
  "foundryURL": "http://127.0.0.1:30000",
  "listen": ":8080",
  "userHeader": "Remote-User",
  "onUnmapped": "fallthrough",
  "trustedProxies": ["10.42.0.0/16"],
  "users": {
    "alice": {
      "foundryUserId": "AbC123dEf456GhI7",
      "passwordFile": "/run/secrets/foundry-alice"
    }
  }
}
```

- `foundryUserId` is the 16-character `_id` of the Foundry user, not the display name.
- `onUnmapped`: what happens when the request has no identity or the identity has no mapped user. `fallthrough` shows the native join page, `deny` returns 403.
- `trustedProxies` (required): CIDRs of the auth proxy. `userHeader` is stripped from requests from any other source.
- Auto-login only happens on `GET /join`. It is skipped when `?manual` is present or the request comes from `/game` (Foundry's log out), so logging out shows the native join page.

## Development

```sh
nix develop
go test ./...
go run ./cmd/foundry-auth-proxy -config config.json
```
