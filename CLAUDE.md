# CLAUDE.md

Guidance for AI coding assistants (Claude Code, Cursor, etc.) working in this
repository. Keep this file short and factual; update it when the commands or
conventions below change.

## What this repository is

`github.com/teslamotors/vehicle-command` is Tesla's Go SDK for the
end-to-end authenticated vehicle command protocol. It contains:

| Path | Purpose |
| --- | --- |
| `pkg/vehicle` | High-level API (`vehicle.Vehicle`): commands and `GetState` queries. Most callers start here. |
| `pkg/protocol` | Protocol layer: sessions, signing, error classification, protobuf definitions (`protobuf/*.proto`) and generated Go (`protobuf/**/*.pb.go`). `protocol.md` is the protocol specification. |
| `pkg/connector` | Transports: `ble` (Bluetooth Low Energy) and `inet` (Fleet API over HTTPS). |
| `pkg/proxy` | HTTP proxy that converts REST calls into signed vehicle commands. |
| `pkg/account`, `pkg/cache`, `pkg/cli`, `pkg/sign` | Fleet API account/OAuth, session cache, shared CLI flags, JWS signing. |
| `internal/` | `dispatcher` (request/response routing and retries), `authentication`, `schnorr`, `log`. Not importable by external modules. |
| `cmd/` | Binaries: `tesla-control`, `tesla-http-proxy`, `tesla-keygen`, `tesla-auth-token`, `tesla-jws`. |
| `examples/` | Small end-to-end programs. |

## Toolchain and commands

* Go version is pinned by `go.mod` (`go 1.23`); CI uses Go 1.23.0. Go 1.22
  cannot auto-download a toolchain for a patch-less `go 1.23` directive, so
  install Go 1.23.x explicitly if the local `go` is older.
* golangci-lint is pinned to **v1.61.0** in `.github/workflows/build.yml`;
  config is `.golangci.yml`.

Run before opening a PR (this mirrors CI):

```bash
go build ./...
go test -cover ./...
go vet ./...
gofmt -l .                                      # must print nothing
golangci-lint run --exclude-use-default=false   # or: make linters
```

`./check-all.sh` runs build, test, vet, and the gofmt check in one go.
`make test` runs `go install ./cmd/...` first; `make format` also rewrites
`pkg/account/version.txt` from the latest git tag, so do not commit that
file unless you are cutting a release.

Regenerate protobuf code only when a `.proto` changes:

```bash
make proto-gen   # requires protoc + protoc-gen-go
```

## Code conventions

* Standard Go style; `gofmt` is enforced. Document any exported identifier
  you add (revive's `exported` rule is disabled in `.golangci.yml` only
  because of pre-existing gaps).
* Errors returned to callers should be classifiable:
  * Protocol-layer faults from the vehicle are `*protocol.RoutableMessageError`
    with a `Code` (`universal.MessageFault_E`). Never replace them with plain
    strings; callers rely on `errors.As`.
  * Use `protocol.NewError(msg, mayHaveSucceeded, temporary)` for library
    errors so `protocol.ShouldRetry`, `MayHaveSucceeded`, and `Temporary` keep
    working. `Vehicle.Send` retries only when `ShouldRetry` is true.
  * Application-layer refusals from the car are `*protocol.NominalError`.
  * Wrap with `fmt.Errorf("...: %w", err)`; do not swallow the original.
* Do not change wire formats (`.proto` files, signature metadata, counters)
  without reading `pkg/protocol/protocol.md`. Vehicles run firmware you cannot
  update; the client must stay compatible.
* BLE responses are bounded by a vehicle-side size limit (see "Response size
  limits" in `protocol.md`). Do not add client-side workarounds that disable
  response encryption or shorten UUIDs to squeeze under it.
* Tests live beside the code (`*_test.go`), use the standard `testing`
  package, and prefer table-driven cases. `pkg/vehicle/vehicle_test.go`
  provides `newTestVehicle()` and a fake dispatcher (`testSender`) for
  exercising `Vehicle` without hardware; `fixedResponse` returns the same
  `RoutableMessage` for every request.
* Never commit private keys, OAuth tokens, or VINs. Test keys used in
  `protocol.md` are public and intentionally throwaway.

## Commits and pull requests

* Commit subjects are imperative and usually scoped by package, e.g.
  `proxy: return 408 for asleep vehicles`, `protocol: fix typos in
  protocol.md`. One logical change per commit.
* Fill in `PULL_REQUEST_TEMPLATE.md`: summary, linked issue
  (`Fixes #NNN`), type of change, and the checklist (style, self-review,
  docs, tests).
* CI (`Build and Test`) runs on every PR: gofmt diff check, golangci-lint,
  `make test`. A PR is not ready until all three pass locally.
* Security issues go to https://www.tesla.com/legal/security, not to GitHub
  issues (see `SECURITY.md`).

## Things an assistant should not do here

* Do not run `make format`/`make build` and commit the resulting
  `pkg/account/version.txt` change.
* Do not edit generated `*.pb.go` files by hand.
* Do not add third-party dependencies for convenience; `go.mod` is small
  and the library is consumed by other projects.
* Do not send commands to a real vehicle while testing unless the user has
  explicitly provided one and asked for it.
