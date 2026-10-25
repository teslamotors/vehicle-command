# vehicle-command

Go library and tools for Tesla Fleet API vehicle commands (HTTP proxy, BLE, signed commands).

## Layout

- `pkg/account` — OAuth account, Fleet Auth token exchange, scope helpers
- `pkg/vehicle` — command API
- `pkg/proxy` — `tesla-http-proxy`
- `pkg/connector` — inet / BLE transports
- `cmd/*` — CLIs (`tesla-http-proxy`, `tesla-control`, `tesla-keygen`, `tesla-auth-token`)

## Conventions

- Small, explicit diffs. Match existing names, errors, and HTTP helpers.
- Do not log tokens, client secrets, or authorization codes.
- Bound response bodies (`connector.MaxResponseLength`).
- Tests use `httptest`; do not call Tesla in unit tests.

## OAuth / `vehicle_location`

Tesla requires `vehicle_location` for location data. It is not implied by `vehicle_device_data`.

Token exchange (`POST /oauth2/v3/token` on `fleet-auth.prd.vn.cloud.tesla.com`) must send `scope` including `vehicle_location`. Use `account.DefaultThirdPartyScopes` and `account.Exchange`. If Tesla omits a requested resource scope, `Exchange` returns `account.MissingScopeError` (tokens still returned so refresh tokens can be saved).

`account.New` exposes JWT `scp` on `Account.Scopes`.

## Checks

```
make format
make test
make linters
```
