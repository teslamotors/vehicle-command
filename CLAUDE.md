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
  update; the client must stay compatible. Additive optional fields (new
  `oneof optional_*` members on an existing message) are the only proto change
  that is backward compatible: older firmware ignores unknown fields. Never
  reuse or renumber existing fields. After editing a `.proto`, regenerate with
  `make proto-gen` using the `protoc` / `protoc-gen-go` versions recorded in
  the generated file header. Tent mode (`SetTentModeRequestAction`, field 94)
  and suspension height (`SetSuspensionLevelAction`, field 118) fill unused
  VehicleAction numbers; do not reuse them.
* Key display names are Fleet API account metadata (`Account.UpdateKey` /
  `api/1/users/keys`), not a VCSEC field. Do not add a name string to
  `KeyMetadata`. BLE can update role and form factor via
  `WhitelistOperation.updateKeyAndPermissions` (`Vehicle.UpdateKeyMetadata`).
  Callers that ask to rename over BLE get `protocol.ErrKeyNameRequiresFleetAPI`.
* WiFi enable/add/forget/connect-in-drive is in-car UX only. Tesla has not
  published VehicleAction field numbers. Do not invent unused oneof tags
  (PSK leakage / firmware collision). Return `protocol.ErrWiFiNotInProtocol`.
  Telemetry is teslamotors/fleet-telemetry#407, not this repo.
* There is no published VehicleAction to keep infotainment awake. `wake`
  only starts infotainment; `SetKeepAccessoryPowerMode` does not power the
  glovebox dashcam USB. Do not invent a keep-alive oneof or wrap
  `charge-port-close` as keep-awake. Return
  `protocol.ErrKeepAwakeNotInProtocol`. See teslamotors/vehicle-command#397.
* `charge_stop` and `set_charging_amps` are published Infotainment
  VehicleActions. Fleet Telemetry (`ACChargingPower`, `Soc`) can look live
  while Tesla `signed_command` returns `vehicle unavailable: vehicle is
  offline or asleep` (`inet.ErrVehicleNotAwake`) because charging hardware
  can run with Infotainment asleep. `wake` does not inhibit later sleep.
  Do not invent keep-awake, a charging-controller bypass, or treat
  telemetry as proof Infotainment will accept the command. Return
  `protocol.ErrChargingWhileInfotainmentAsleep` for clients that ask this
  SDK to do that. See teslamotors/vehicle-command#452.
* `GET /api/1/vehicles` `state=online` and a successful VCSEC
  `signed_command` do not mean `DOMAIN_INFOTAINMENT` will accept
  `session_info_request`. Tesla's gateway can return HTTP 408
  `vehicle is offline` (`inet.ErrVehicleNotAwake`) for Infotainment while
  the list says online, VCSEC works, BLE reaches both domains, and the
  operator is in the car (reported on `fleet-api.prd.cn`). Both domains
  share one `signed_command` POST; the domain is inside the protobuf.
  Do not rewrite the domain to VCSEC, skip the Infotainment handshake
  (#468 / #470), or treat `wake_up` as a guarantee. Return
  `protocol.ErrInfotainmentSignedCommandOffline` for clients that ask this
  SDK to do that. See teslamotors/vehicle-command#285.
* Battery pack identity (`$BT*` option codes) is Tesla catalog metadata
  (`GET /api/1/dx/vehicles/options`), not a signed vehicle field. Tesla
  omits `bt` for many VINs. Do not invent `$BT*` codes from model codes.
  Return `protocol.ErrBatteryOptionNotInCatalog` when absent.
  `ChargeState` has SOC/range, not pack kWh. Partner
  `/api/1/vehicles/{vin}/specs` (`batteryCapacityKwh`) is billed. See
  teslamotors/vehicle-command#391.
* HTTP handlers must derive per-request timeouts from `req.Context()`, not
  `context.Background()`, so client disconnects cancel pending Fleet API and
  vehicle lookup work. `Dispatcher.Start` must `Stop` the listener if that
  context is canceled before the handler defers `Disconnect`. See
  teslamotors/vehicle-command#491. Cancellation does not recall a command
  already delivered to the vehicle.
* Dispatcher.StartSessions launches one worker per domain and returns on the
  first non-Canceled handshake error. Buffer the results channel to
  `len(domains)` (after expanding the default domain list) so remaining
  workers cannot block forever on send. See teslamotors/vehicle-command#494.
* An enrolled BLE client is a VCSEC whitelist key. Role gates commands;
  form factor is display metadata; neither is a published "ignore for
  Walk-Away Door Lock" flag. Do not invent a command-only presence-exempt
  whitelist field. In-car gadgets must `Disconnect` after each command so
  they are not treated as a phone key left inside. Return
  `protocol.ErrBLEKeyPresenceNotInProtocol`. See
  teslamotors/vehicle-command#480.
* Scheduled charging and scheduled departure commands are published and
  delivered. Whether the vehicle later sleeps and fires the scheduler is
  firmware. Cabin overheat protection can block that cycle on some
  Intel-MCU Model S vehicles. Do not disable cabin overheat as a library
  workaround. Return `protocol.ErrScheduledChargingFirmware`. See
  teslamotors/vehicle-command#342.
* Fleet API lists `remote_boombox`, but Tesla has not published a
  VehicleAction for the external speaker. A collaborator stated legal
  review of Pedestrian Warning System restrictions is required
  (teslamotors/vehicle-command#266). Honk and flash-lights are published.
  Do not invent an unused oneof tag, copy a third-party firmware dump, or
  map boombox onto honk. Return `protocol.ErrBoomboxNotInProtocol`. See
  teslamotors/vehicle-command#266 and #411.
* `HvacAutoAction` is climate power (`ClimateOn` / `ClimateOff`, Fleet
  `auto_conditioning_start` / `stop`), not Auto vs Manual HVAC.
  `manual_override` is a low-SOC override. Tesla has not published a
  VehicleAction for Auto vs Manual (or heater-off / vent). Return
  `protocol.ErrHvacAutoModeNotInProtocol`. Set temperatures with
  `driver_temp_celsius` / `passenger_temp_celsius`
  (`ChangeClimateTemp` / `set_temps`); `absolute_celsius` alone is proto3
  0 on those fields and firmware treats that as LO. Do not send
  `TEMP_MAX` with a numeric setpoint. See teslamotors/vehicle-command#283.
* Climate split / SYNC (linked vs independent driver and passenger HVAC)
  is in-car UX. Tesla has not published a VehicleAction or a
  ClimateState split boolean. Independent setpoints are already
  `HvacTemperatureAdjustmentAction` (`ChangeClimateTemp` / `set_temps`).
  `GetClimateState` returns `driver_temp_setting` and
  `passenger_temp_setting`; do not invent an `is_climate_split` field
  or unused oneof. Return `protocol.ErrClimateSplitNotInProtocol`. See
  teslamotors/vehicle-command#386.
* `https://tesla.com/_ak/<domain>` is Tesla's hosted virtual-key page.
  The Finish Setup button is not in this SDK. Do not append an
  unconstrained `return_uri` (open redirect). A redirect would have to
  stay on the registered partner domain and be configured with Tesla.
  `account.VirtualKeyInstallURL` builds the link with no query string.
  Return `protocol.ErrVirtualKeyReturnURI` when a caller asks to set
  `return_uri`. See teslamotors/vehicle-command#444.
* `set_climate_keeper_mode` (Dog=2, Camp=3) is published
  `HvacClimateKeeperAction`. Firmware may refuse with NominalError
  `cpd_enabled` (Child Presence Detection occupancy radar). That is not
  the in-car Child Left Alone Detection setting. `manual_override` is a
  low-SOC override, not a CPD bypass (teslamotors/vehicle-command#437).
  Do not invent a CPD-disable oneof or wrap climate-on as Dog Mode.
  Deliver the published action; return
  `protocol.ErrClimateKeeperCPDFirmware` for clients that ask this SDK
  to bypass CPD. Live refusals stay `*protocol.NominalError` (HTTP 200
  `result:false`). See teslamotors/vehicle-command#509.
* Charging Manager (`ROLE_CHARGING_MANAGER`) authorizes charging
  start/stop/amps. Charge-port open/close is firmware-gated
  (`MESSAGEFAULT_ERROR_INSUFFICIENT_PRIVILEGES`). Deliver
  `ChargePortDoorOpen`/`Close`; do not enroll Owner, invent KeyMetadata
  permissions, or bypass via VCSEC `ClosureMoveRequest.chargePort`. Return
  `protocol.ErrChargingManagerChargePortFirmware` for clients that ask this
  SDK to expand that ACL. See teslamotors/vehicle-command#413.
* BLE `GetState` / `GetDriveState` latency is the vehicle round-trip
  (~250–300ms observed). Construction and encryption are a few
  milliseconds. There is no published streaming DriveState action. Do not
  disable `FLAG_ENCRYPT_RESPONSE`, shorten UUIDs, or skip the handshake to
  chase <150ms. Handshake once and reuse the session; each extra category
  is another RTT. High-rate telemetry is teslamotors/fleet-telemetry.
  Return `protocol.ErrBLEStateLatencyFirmware`. See
  teslamotors/vehicle-command#414.
* `remote_seat_heater_request` / `remote_seat_cooler_request` already map
  to published `HvacSeatHeaterActions` (field 36) and
  `HvacSeatCoolerActions` (field 49). Accept Fleet/Owner JSON `heater` as
  an alias for `seat_position` on the heater path. Tesla
  `signed_command` HTTP 501 with JSON `Unauthorized` is Fleet API
  partner/region/OAuth allowlist; `writeJSONError` forwards Tesla's
  status so clients see "Not Implemented". That is not a missing handler
  and not a reason to invent VehicleAction numbers. Return
  `protocol.ErrSeatClimateFleetAPI` for clients that ask this SDK to
  treat those paths as unimplemented. See teslamotors/vehicle-command#383.
* Tesla Fleet Auth `invalid_audience` on `client_credentials` and
  `/authorize` "No policy rules" are Tesla Identity Provider
  provisioning, not a missing VehicleAction. `tesla-auth-token` stores a
  token the caller already obtained; `account.New` reads JWT `aud` of
  that token. This SDK does not mint partner tokens, bind audiences, or
  `POST /api/1/partner_accounts` without a token. Do not invent an
  audience, retry regional Fleet API hosts as a workaround, or commit
  client secrets. Return `protocol.ErrPartnerOAuthNotProvisioned`.
  Direct dashboard/OAuth provisioning questions to Tesla developer
  Support Inquiry. See teslamotors/vehicle-command#460.
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
