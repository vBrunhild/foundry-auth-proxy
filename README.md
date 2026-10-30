# Foundry Auth Proxy

Reverse proxy for Foundry VTT that skips the native `/join` screen. It trusts an identity header set by an upstream auth proxy, maps that identity to a Foundry user, and replays Foundry's `POST /join` on the user's behalf, relaying the session cookie.

Foundry sessions are in-memory and cannot be created externally, so every mapped Foundry user needs a real access key, kept in a users file readable by the proxy.

## Configuration

```json
{
  "foundryURL": "http://127.0.0.1:30000",
  "listen": ":8080",
  "userHeader": "Remote-User",
  "onUnmapped": "fallthrough",
  "trustedProxies": ["10.42.0.0/16"],
  "usersFile": "/secrets/users.json"
}
```

- `foundryURL` (required): absolute URL of the Foundry server. Every request is proxied to it, and logins are replayed against its `/join` endpoint. When both run in the same pod this is `http://127.0.0.1:30000`.
- `listen`: address the proxy listens on, as `host:port`. Defaults to `:8080` (all interfaces).
- `userHeader`: request header that carries the authenticated username, set by the upstream auth proxy (Authelia sends `Remote-User`). Its value is looked up as a key in the users file. Defaults to `Remote-User`.
- `onUnmapped`: what happens when the request has no identity or the identity has no mapped user. `fallthrough` shows the native join page, `deny` returns 403.
- `trustedProxies` (required): CIDRs of the auth proxy. `userHeader` is stripped from requests from any other source.
- `usersFile` (required): is re-read on every login, so edits and secret rotations apply without a restart.

The users file maps the identity from `userHeader` to a Foundry user:

```json
{
  "alice": { "foundryUserId": "AbC123dEf456GhI7", "password": "alices-access-key" }
}
```

- `foundryUserId`: the 16-character `_id` of the Foundry user, not the display name.
- `password`: the Foundry user's access key, the same one they would type on the native join screen. The proxy sends it to Foundry's `/join` endpoint, so keep the users file readable only by the proxy.

## Development

```sh
nix develop
go test ./...
go run ./cmd/foundry-auth-proxy -config config.json
```
