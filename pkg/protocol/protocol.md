# Protocol Specification

This document contains a specification of the protocol used by the mobile app
to send commands to Tesla vehicles. It covers session management and
cryptography. Application-layer payloads are out of scope, but should be easily
reproduced by following source code links from the [package
documentation](https://pkg.go.dev/github.com/teslamotors/vehicle-command/pkg/vehicle).

## Message encoding

Protocol messages are encoded using Google's [Protocol
Buffers](https://protobuf.dev/). The top-level type is a `RoutableMessage`,
defined in [universal_message.proto](protobuf/universal_message.proto).

A typical RoutableMessage looks like this:

```proto
to_destination {
  domain: DOMAIN_VEHICLE_SECURITY
}
from_destination {
  routing_address: 0a7962c10d38b61dd2a7722780a4f096
}
protobuf_message_as_bytes: 0a020805
uuid: 05514f57616bcc81a8ce0f9d7b483229
```

The fields of a RoutableMessage are always raw binary values; the hex encodings
above are for printability.

 * For commands sent to the vehicle, the
   `to_destination` should be the [Domain](#Domains) responsible for handling
   the command.
 * For commands sent to the vehicle, `from_destination` should be a randomly
   generated 16-byte `routing_address` that uniquely identifies the connection.
   Vehicles use routing addresses to associate clients with transport-layer
   addresses when forwarding the response.
 * The vehicle reverses the to and from fields in its response.
 * The `protobuf_message_as_bytes` field is a
   [`oneof`](https://protobuf.dev/programming-guides/proto3/#oneof) named
   `payload` that can have the following types:
   * `payload.protobuf_message_as_bytes` contains an application-layer payload.
     Messages addressed to `DOMAIN_VEHICLE_SECURITY` (VCSEC) parse this as a
     [`vcsec.UnsignedMessage`](protobuf/vcsec.proto), while messages addressed to
     `DOMAIN_INFOTAINMENT` parse this as a
     [`carserver.Action`](protobuf/car_server.proto).
   * `payload.session_info_request` is a [handshake request](#Request) from a
     client. The client needs to complete a handshake before it can send commands.
   * `payload.session_info` is a [handshake response](#Response) from the vehicle.
 * The `signature_data` field contains information used to authenticate the
   command. See [Authentication](#Authentication-methods).
 * The `uuid` field must uniquely identify the command, and must be at most 16
   bytes. It will be copied into the `request_uuid` field of the response. Due
   to memory constraints, the `request_uuid` field is typically not populated
   in replies from VCSEC. Requests can use unique routing addresses to
   disambiguate responses, but note that due to VCSEC memory constraints,
   clients should avoid making simultaneous requests to that domain. The UUID
   should be unpredictable in order to prevent replayed handshake responses.
 * The vehicle sets `signedMessageStatus` to indicate protocol-layer errors.
   [Application-layer errors](#Response-handling) appear in
   `protobuf_message_as_bytes`.
 * The `flags` field is a bit mask of `universal_message.Flags` values.
   Vehicles authenticate this value, but ignore unrecognized bits. Clients
   should always set the `FLAG_ENCRYPT_RESPONSE` bit, which instructs vehicles
   with compatible firmware (2024.38+) to encrypt the response.

## Decoding messages

For command-line debugging, a RoutableMessage can be decoded using the
[protoc](https://grpc.io/docs/protoc-installation/) tool from Google. It can
also [generate language-specific bindings](https://protobuf.dev/) from the
`*.proto` files [included in this repository](./protobuf).

For example, running `tesla-control -vin YOUR_VIN -ble -debug list-keys` shows
a debug line:

```
2023-12-13T14:41:13-08:00 [debug] TX: 320208023a1212100a7962c10d38b61dd2a7722780a4f0969a031005514f57616bcc81a8ce0f9d7b48322952040a020805
```

The hex string above can be decoded using `protoc` using the command below,
executed from this document's directory:

```bash
echo 320208023a1212100a7962c10d38b61dd2a7722780a4f0969a031005514f57616bcc81a8ce0f9d7b48322952040a020805 \
    | xxd -r -p \
    | protoc --decode=UniversalMessage.RoutableMessage -I protobuf protobuf/*.proto
```

```proto
to_destination {
  domain: DOMAIN_VEHICLE_SECURITY
}
from_destination {
  routing_address: "\nyb\301\r8\266\035\322\247r\'\200\244\360\226"
}
protobuf_message_as_bytes: "\n\002\010\005"
uuid: "\005QOWak\314\201\250\316\017\235{H2)"
```

This shows the client transmitted (TX) a RoutableMessage sent to
`DOMAIN_VEHICLE_SECURITY`. If a message is sent to this domain, then the
`protobuf_message_as_bytes` field encodes a VCSEC.UnsignedMessage, which can be
similarly decoded with `protoc`:

```bash
printf "\n\002\010\005" | protoc --decode=VCSEC.UnsignedMessage -I protobuf protobuf/*.proto
```

```proto
InformationRequest {
  informationRequestType: INFORMATION_REQUEST_TYPE_GET_WHITELIST_INFO
}
```

## Message transports

Messages can be sent to vehicles either over a REST API or over BLE.

### HTTPS

See [Fleet API
documentation](https://developer.tesla.com/docs/fleet-api/getting-started/what-is-fleet-api) for
information on using OAuth authentication.

To send a message to a vehicle, make a POST request to
`api/1/vehicles/<vin>/signed_command`. The body should be `{"routable_message":
<base64-encoded RoutableMessage>}`.

If the server receives a response from the vehicle, it returns status code 200
and the response body will be a JSON document of the form `{"response":
<base64-encoded RoutableMessage>}`.

Note that a 200 HTTP code does not indicate the vehicle successfully executed
a command, just that the server received a response. The `RoutableMessage` may
contain an error message.

See [online
documentation](https://developer.tesla.com/docs/fleet-api/getting-started/conventions#response-codes)
for information on other HTTP status codes.

Although communication between clients and Tesla's servers use TLS/TCP, the
communication channel between Tesla's servers and vehicles does not provide TCP
transport guarantees; **messages may be dropped or arrive out of order**.

### BLE

Clients connect to vehicles using the following identifiers:

 * Service UUID: `00000211-b2d1-43f0-9b88-960cebf8b91e`
 * Vehicle write characteristic: `00000212-b2d1-43f0-9b88-960cebf8b91e`. Use
   your BLE library's "write with response" API.
 * Vehicle read characteristic: `00000213-b2d1-43f0-9b88-960cebf8b91e`
 * BLE advertisement local name: `S + <ID> + C`, where `<ID>` is the
   lower-case hex-encoding of the first eight bytes of the SHA1 digest of the
   Vehicle Identification Number (VIN). For example, If the VIN is
   `5YJS0000000000000`, then the BLE advertisement Local Name is
   `S1a87a5a75f3df858C`.

Messages sent or received over BLE are preceded by the two-byte big-endian
encoding of the message length.

*Note*: Due to hardware constraints, VCSEC can only reliably maintain up to
three simultaneous BLE connections. These are shared by keyfobs and phone keys.

### BLE keys and Walk-Away Door Lock

Enrolling a public key with `add-key-request` / `Vehicle.AddKey` puts that
key on the VCSEC **whitelist**. From the vehicle's point of view it is a
key, the same whitelist used by phone keys and keyfobs. Role (Owner,
Driver, …) only decides which **commands** the key may authorize. Form
factor (`nfc_card`, `ios_device`, `android_device`, `cloud_key`) is
`KeyMetadata` display metadata, not a documented exemption from presence
detection.

Tesla's owner documentation treats a paired phone left inside with
Bluetooth enabled as equivalent to leaving keys in the car: Walk-Away Door
Lock does not lock because an authenticated BLE key is still present.
VCSEC has no published field that marks a whitelist entry as
"command-only, ignore for passive-entry / Walk-Away Door Lock." Inventing
one would collide with firmware this repository cannot update.

An in-car BLE button that stays connected, or reconnects on a timer, can
therefore prevent automatic locking. That is expected firmware behavior,
not a library bug. The supported architecture is:

1. Connect over BLE when a command is needed.
2. Handshake, send the command, then `Vehicle.Disconnect` /
   `connector.Close` so the GATT session ends.
3. Do not advertise or reconnect while the vehicle is parked with people
   walking away.

`tesla-control` already disconnects when the process exits. Embedded
controllers must do the same after each button press. The library returns
[`protocol.ErrBLEKeyPresenceNotInProtocol`](error.go) for
`tesla-control ble-presence-exempt` and proxy paths `ble_presence_exempt`
/ `command_only_key` rather than guessing a whitelist flag. See
[issue #480](https://github.com/teslamotors/vehicle-command/issues/480).

### BLE GetDriveState / GetVehicleData latency

`GetState` (for example `StateCategoryDrive`: gear, speed) is a signed
Infotainment request/response. Callers looping over BLE have measured
**~250–300ms** of radio plus vehicle time, with client construction and
encryption only a few milliseconds
([issue #414](https://github.com/teslamotors/vehicle-command/issues/414)).
Targets of under 150ms (or 50–100ms) are below that vehicle round-trip.
This SDK has no timer that pads the poll to 250ms, and it cannot shorten
Infotainment processing.

There is **no** published streaming or subscription `VehicleAction` for
DriveState. `GetVehicleData` markers have no field mask that would make
the vehicle answer faster. Disabling `FLAG_ENCRYPT_RESPONSE`, shortening
UUIDs, or skipping the session handshake does not raise a firmware poll
cap and is unsafe (response-size workarounds are also forbidden; see
[Response size limits](#response-size-limits)).

Practical client advice:

1. `StartSession` once on Infotainment and **reuse** the session for each
   poll. Handshake plus GetState is two round-trips (~500ms), which matches
   reports of "250ms × 2".
2. Request **one** `StateCategory` per poll (`tesla-control state drive`).
   Extra categories are extra RTTs.
3. Gear and speed are Infotainment `DriveState`, not VCSEC
   `BodyControllerState` (that path works while infotainment sleeps but
   does not carry speed).
4. High-rate telemetry is [fleet-telemetry](https://github.com/teslamotors/fleet-telemetry),
   not this repository.

Callers that ask this library to guarantee a sub-150ms BLE poll get
[`protocol.ErrBLEStateLatencyFirmware`](error.go) from
`tesla-control ble-state-fast` and proxy paths `ble_state_fast` /
`drive_state_fast` / `set_ble_poll_interval` (HTTP 400 before a session).

### Seat heater / cooler vs Tesla `signed_command` HTTP 501

`remote_seat_heater_request` and `remote_seat_cooler_request` are
**implemented**. tesla-http-proxy maps them to published
`HvacSeatHeaterActions` (VehicleAction field 36) and
`HvacSeatCoolerActions` (field 49). `tesla-control seat-heater` and
`seat-cooler` send those actions over BLE or Fleet API.

Owner/Fleet JSON historically used `"heater"` (0–8) plus `"level"`
(0–3) for the heater endpoint; the proxy also accepts `"seat_position"`
(the same 0–8 index). Cooler uses `"seat_position"` plus
`"seat_cooler_level"` (`"level"` is an alias). Missing parameters are
HTTP 400 **before** a vehicle session.

If Tesla `POST .../signed_command` returns HTTP **501** with JSON
`"error":"Unauthorized"`, tesla-http-proxy **already signed and
forwarded** the command. [`writeJSONError`](../proxy/proxy.go) copies
Tesla's status, so clients see "Not Implemented"
(`http.StatusText(501)`). That is Tesla Fleet API partner/region/OAuth
allowlist, not `proxy.ErrCommandNotImplemented`. Other REST commands
succeeding does not mean these handlers are missing
([issue #383](https://github.com/teslamotors/vehicle-command/issues/383)).

This SDK does not invent unused VehicleAction numbers or skip command
signing. Callers that ask to treat the published paths as unimplemented
get [`protocol.ErrSeatClimateFleetAPI`](error.go) from
`tesla-control seat-heater-not-implemented` and proxy paths
`seat_heater_not_implemented` / `seat_cooler_not_implemented` /
`remote_seat_climate_not_implemented` (HTTP 400 before a session).

### Partner token `invalid_audience` and `/authorize` "No policy rules"

This repository **does not mint Tesla OAuth tokens**.
`tesla-auth-token` writes a token the caller already obtained to the
system keyring. [`account.New`](../account/account.go) reads the JWT
`aud` claim of that token to pick a Fleet API host. That is unrelated
to Tesla Fleet Auth rejecting `grant_type=client_credentials` with
`invalid_audience`, or `GET /oauth2/v3/authorize` showing
"No policy rules".

Those errors mean Tesla's Identity Provider has not bound OAuth
policy/audience to the application, even when the developer dashboard
shows the app as Active and `client_id`/`client_secret` are accepted
(`unauthorized_client` vs `invalid_audience` vs `client_not_found`).
`POST /api/1/partner_accounts` requires a partner token Tesla has not
issued. This SDK does not invent an audience, POST partner
registration without a token, or send client secrets. Retrying NA/EU/CN
`fleet-api` audience URLs does not provision Tesla's IdP. File a
Support Inquiry from the Tesla developer dashboard
([issue #460](https://github.com/teslamotors/vehicle-command/issues/460)).

Callers that ask this library to obtain a partner token or bind
audiences get [`protocol.ErrPartnerOAuthNotProvisioned`](error.go) from
`tesla-control partner-oauth` and proxy paths `partner_token` /
`register_partner` / `oauth_audience` / `invalid_audience` (HTTP 400
before a session).

## Protocol concepts

This section provides an overview of concepts handled by the protocol.

### Domains

Vehicles have subsystems called _domains_ that have different public keys and
therefore require separate handshakes and session state tracking. The VCSEC
domain controls locks, remote start, and trunk, among others; The Infotainment
domain processes the remaining commands. VCSEC can be reached over BLE even
when infotainment is asleep.

### Time

Each domain has its own clock and represents time using `(epoch_id,
timestamp)` pairs. The `epoch_id` is a random 16-byte value generated at boot,
and the `timestamp` is an integer giving the number of seconds since the start
of the epoch.

### Roles

The vehicle associates each client public key with a _role_ that
determines what commands that client can authorize. The most common roles are
Owner and Driver. Roles are enumerated in [keys.proto](protobuf/keys.proto).

The capabilities of a given role may change as new features are added or new
use-cases arise.

An **Owner** can authorize all commands, including adding and removing public
keys for other users.

A **Driver** has access to most commands but cannot manage other users' keys or
otherwise configure access controls, such as changing vehicle PINs.

A **Fleet Manager** represents a cloud-based Owner key. In vehicles running
2023.38 or later, a Fleet Manager cannot add or remove other users' keys from
the vehicle and cannot send commands over BLE. If a cloud-based service needs
to manage Owner and Driver access, this should be done at the account level
using [Fleet API](https://developer.tesla.com/docs/fleet-api/endpoints/vehicle-commands).

A **Vehicle Monitor** can read vehicle data, such as location information, but
cannot authorize commands that change the vehicle's state.

A **Charging Manager** can read vehicle data and authorize commands that
affect vehicle charging (`charging-start` / `stop` / `set-amps` work over
BLE). Role enforcement is firmware, not this SDK. `ChargePortDoorOpen` /
`ChargePortDoorClose` are published Infotainment actions and are still
delivered (`Vehicle.OpenChargePort`). Vehicles have historically refused
those for Charging Manager keys with
`MESSAGEFAULT_ERROR_INSUFFICIENT_PRIVILEGES`
([issue #232](https://github.com/teslamotors/vehicle-command/issues/232)).
Tesla has not expanded that ACL. This SDK does not enroll Owner for a
charge-door gadget, invent extra `KeyMetadata` permissions, or rewrite the
command as a VCSEC `ClosureMoveRequest.chargePort` bypass. Callers that
ask for that workaround get
[`protocol.ErrChargingManagerChargePortFirmware`](error.go). See
[issue #413](https://github.com/teslamotors/vehicle-command/issues/413).

A **Guest** key is essentially a temporary Driver key, with an automated
lifecycle intended to facilitate vehicle rentals. See
[Fleet API documentation](https://developer.tesla.com/docs/fleet-api/endpoints/vehicle-commands#guest-mode).

A **Service** key bootstraps pairing other keys and authorizes commands on
behalf of service technicians. Service keys can remotely (un)lock vehicles in
order to provide roadside assistance, as well as remotely delete (but not add)
Driver, Guest, and Fleet Manager keys. Vehicles in their default state prevent
Service keys from authorizing other commands over the Internet.

### Key display names

The Locks screen shows a human-readable label for each enrolled key. That
string is **not** stored on the vehicle. VCSEC [`KeyMetadata`](protobuf/vcsec.proto)
only contains `keyFormFactor`. Tesla's account service holds the label
(`POST api/1/users/keys`, implemented as `Account.UpdateKey` /
`tesla-control rename-key`). Vehicles fetch it when they have connectivity.

BLE-only clients can enroll keys and update role or form factor locally
(`Vehicle.AddKey`, `Vehicle.UpdateKeyMetadata` /
`tesla-control -ble update-key`) but cannot rename the Locks-screen label.
The library returns [`protocol.ErrKeyNameRequiresFleetAPI`](error.go) in that
case. See [issue #418](https://github.com/teslamotors/vehicle-command/issues/418).

### Virtual key enrollment page

`https://tesla.com/_ak/<domain>` is Tesla's hosted page for pairing a
partner public key. `account.VirtualKeyInstallURL` builds that link from
the registered hostname and does not add a query string. The Finish Setup
button is part of Tesla's page. On a desktop browser it can return to the
same page.

This repository does not host that page and cannot retarget the button.
An unconstrained `return_uri` (for example `?return_uri=https://other.example/finish`)
is an open redirect. A Tesla collaborator said any such redirect must stay
on the registered partner domain and/or be configured with Tesla in advance
([issue #444](https://github.com/teslamotors/vehicle-command/issues/444)).
Callers that ask this SDK to append `return_uri` get
[`protocol.ErrVirtualKeyReturnURI`](error.go) from
`tesla-control virtual-key-return` and proxy paths `virtual_key_return` /
`ak_return_uri` / `set_virtual_key_return` (HTTP 400 before a session).
`account.VirtualKeyReturnHostAllowed` reports whether an https URL's host
is the partner domain or a subdomain. That check does not make Tesla's
page follow the URL.

### WiFi configuration

The in-car UX can enable WiFi, add a network (SSID, security, PSK), forget a
network, and toggle connect-in-drive. Tesla has **not** published
`VehicleAction` members for those operations. They are also absent from
independent public firmware dumps of `car_server.proto`. Filling an unused
oneof number would collide with firmware Tesla cannot update from this
repository, and could put a PSK on the wire under the wrong tag.

The SDK therefore does not guess field numbers. `protocol.ErrWiFiNotInProtocol`
is returned for `tesla-control wifi` and for proxy paths `wifi_on`,
`wifi_off`, `set_wifi`, `add_wifi_network`, `forget_wifi_network`, and
`wifi_connect_in_drive` (HTTP 400 before a session is opened; the PSK is not
forwarded). Connectivity *telemetry* (SSID, RSSI, cellular) belongs in
[fleet-telemetry#407](https://github.com/teslamotors/fleet-telemetry/issues/407),
not this command SDK. See [issue #419](https://github.com/teslamotors/vehicle-command/issues/419).

### Keeping infotainment awake

`wake` (`RKE_ACTION_WAKE_VEHICLE` over BLE, or Fleet API wakeup over inet)
starts infotainment if it is asleep. It does not inhibit subsequent sleep.
Repeating `wake` has the same limitation. `body-controller-state` talks to
VCSEC and does not keep infotainment awake.

`SetKeepAccessoryPowerMode` (`tesla-control keep-accessory-power`) is the
published setting for the 12V jack and charging USB ports. Tesla firmware
does not apply it to the glovebox dashcam/data USB port.

Tesla has **not** published a `VehicleAction` that keeps infotainment (or the
dashcam USB) powered for USB offload. Inventing an unused oneof number would
collide with firmware. The SDK therefore returns
[`protocol.ErrKeepAwakeNotInProtocol`](error.go) for `tesla-control keep-awake`
and proxy paths `keep_awake` / `keep_alive` (HTTP 400 before a session).
Wrapping an Infotainment mutation (for example `charge-port-close`) as a
library keep-alive is not supported: it changes vehicle state and fights
designed sleep. See [issue #397](https://github.com/teslamotors/vehicle-command/issues/397).

### Charging commands while Infotainment is asleep

`charge_stop` / `Vehicle.ChargeStop` and `set_charging_amps` /
`Vehicle.SetChargingAmps` are published Infotainment `VehicleAction`s
(`ChargingStartStopAction` stop, `SetChargingAmpsAction`). tesla-http-proxy
already maps those Fleet REST paths. Paid Fleet API usage is not a quota on
this path.

[Fleet Telemetry](https://github.com/teslamotors/fleet-telemetry)
(`ACChargingPower`, `Soc`) is a separate product. Charging hardware can
continue while Infotainment is asleep, so telemetry can look live while
Tesla's `signed_command` gateway returns HTTP 408 or 503
`vehicle unavailable: vehicle is offline or asleep`
([`inet.ErrVehicleNotAwake`](../connector/inet/inet.go)). `wake` starts
Infotainment but does not inhibit later sleep (see above) and does not
guarantee Tesla's command relay is ready for the next signed command.

There is no published `VehicleAction` that talks to the charging controller
while Infotainment is unreachable. BLE still delivers these actions to
Infotainment after handshake; it is not a VCSEC charging-amps command.
Inventing keep-awake or wrapping `charge-port-close` as a library workaround
is not supported.

Callers that ask this SDK to treat telemetry as proof `signed_command` will
succeed get [`protocol.ErrChargingWhileInfotainmentAsleep`](error.go) from
`tesla-control charging-while-asleep` and proxy paths `charging_while_asleep`,
`charge_stop_asleep`, and `set_charging_amps_asleep` (HTTP 400 before a
session). See [issue #452](https://github.com/teslamotors/vehicle-command/issues/452).

### Infotainment `signed_command` while the vehicle list says online

`POST /api/1/vehicles/{vin}/signed_command` carries one `RoutableMessage`.
`DOMAIN_INFOTAINMENT` and `DOMAIN_VEHICLE_SECURITY` are fields inside that
protobuf. This SDK does not have a separate HTTP path per domain.

`GET /api/1/vehicles/{vin}` with `state=online` is the vehicle-list state.
It does not mean Tesla's command gateway will accept a message addressed to
Infotainment. A successful VCSEC `signed_command` (unlock, open door) does
not prove the next Infotainment `session_info_request` will be accepted.
BLE is a different transport: both domains can answer there while Fleet API
returns HTTP 408 `vehicle is offline` for Infotainment only.

[Issue #285](https://github.com/teslamotors/vehicle-command/issues/285)
reports that 408 while the list says online, VCSEC commands succeed, the
operator is in the car, and the host is `fleet-api.prd.cn.vn.cloud.tesla.cn`.
The 408 body is Tesla's gateway
([`inet.ErrVehicleNotAwake`](../connector/inet/inet.go)). This repository
cannot make the gateway accept the message. It does not rewrite
`DOMAIN_INFOTAINMENT` to VCSEC, skip the Infotainment handshake
([issue #468](https://github.com/teslamotors/vehicle-command/issues/468)),
or treat `wake_up` as a promise that the next Infotainment handshake
succeeds. `wake_up` remains the unsigned REST wake.

Callers that ask this SDK to treat `state=online` or a VCSEC success as
Infotainment reachability get
[`protocol.ErrInfotainmentSignedCommandOffline`](error.go) from
`tesla-control infotainment-offline` and proxy paths `infotainment_offline`,
`domain_infotainment_online`, and `vcsec_means_infotainment_online`
(HTTP 400 before a session). Published Infotainment commands such as
`honk_horn` are still sent.

### Battery option codes

Pack identity (`$BT42`, `$BTF0`, …) is Tesla **catalog** metadata from
[`GET /api/1/dx/vehicles/options?vin=`](https://developer.tesla.com/docs/fleet-api/endpoints/vehicle-management),
not a signed `VehicleAction` or `ChargeState` field. `ChargeState` reports
SOC and rated range, not pack kWh.

Tesla's options response for many VINs omits any `$BT*` code (see the sample
in [issue #391](https://github.com/teslamotors/vehicle-command/issues/391)).
This SDK does not invent a battery code from a model option such as `$MT322`.
`account.FindBatteryOption` / `tesla-control battery-option VIN` return
[`protocol.ErrBatteryOptionNotInCatalog`](error.go) in that case.
`tesla-control -ble options` returns
[`protocol.ErrBatteryOptionRequiresFleetAPI`](error.go). Proxy paths
`battery_size`, `get_battery_option`, and `get_battery_size` return that
Fleet-required error as HTTP 400 before a vehicle session.

Tesla's documented kWh alternative is partner-token
`GET /api/1/vehicles/{vin}/specs` (`batteryCapacityKwh`). That endpoint is
billed ($0.10 per successful result) and is not called by this SDK.

### Scheduled charging and cabin overheat protection

`ScheduledChargingAction` (field 41) and `ScheduledDepartureAction` (field
42) are published. `Vehicle.ScheduleCharging` /
`tesla-control charging-schedule` and `Vehicle.ScheduleDeparture` /
proxy `set_scheduled_departure` deliver those commands. `charge-start`
starts charging immediately and is a separate action.

Whether the vehicle later **sleeps and fires the scheduler** is firmware,
not a missing client field. On some Intel-MCU Model S vehicles, cabin
overheat protection (`SetCabinOverheatProtectionAction`, field 50) can
prevent that sleep/wake cycle, so scheduled departure or scheduled
charging does not run at the configured time even though the command was
accepted. Turning cabin overheat off restores the schedule on those cars.
Newer Model Y firmware is reported to schedule correctly with cabin
overheat on. See [issue #342](https://github.com/teslamotors/vehicle-command/issues/342).

This SDK does not disable cabin overheat as a workaround (that would
change climate settings the driver enabled) and does not invent a
"force schedule despite overheat" VehicleAction. Callers that ask for
that workaround get [`protocol.ErrScheduledChargingFirmware`](error.go)
from `tesla-control charging-schedule-overheat` and proxy paths
`scheduled_charging_overheat` / `force_scheduled_charging` (HTTP 400
before a session).

### Remote boombox (external speaker)

Fleet API documents `POST .../command/remote_boombox` with `{"sound": 0}`
(random) or `{"sound": 2000}` (locate ping). Vehicles that require the
Vehicle Command Protocol reject **unsigned** REST with HTTP 403.

The published [`car_server.proto`](protobuf/car_server.proto) in this
repository has `VehicleControlHonkHornAction` (field 27) and
`VehicleControlFlashLightsAction` (field 26), but **no** boombox /
play-sound `VehicleAction`. Tesla has not added one. A Tesla collaborator
stated on [issue #266](https://github.com/teslamotors/vehicle-command/issues/266)
that boombox stays unpublished pending legal approval and a disclaimer:
Pedestrian Warning System use is restricted in some jurisdictions.
Third-party firmware dumps and closed-source apps are **not** Tesla's
published protocol. Copying an unpublished oneof member into this
repository would collide with firmware this SDK cannot update and would
release code Tesla has not cleared.

Mapping boombox onto honk would change vehicle behavior (horn vs external
speaker). Locate ping is also firmware-gated (for example robotaxi /
speed limits reported by third parties); this SDK does not guess those
rules.

The proxy therefore returns [`protocol.ErrBoomboxNotInProtocol`](error.go)
for `remote_boombox` (HTTP 400 before a session) instead of a generic
"command not implemented". `tesla-control boombox` returns the same
error. See [issue #266](https://github.com/teslamotors/vehicle-command/issues/266)
and [issue #411](https://github.com/teslamotors/vehicle-command/issues/411).

### HVAC Auto vs climate power

`HvacAutoAction` (VehicleAction field 10) is climate **power**.
`power_on: true` is Fleet API `auto_conditioning_start`; `power_on: false`
is `auto_conditioning_stop`. The protobuf name is historical. It is **not**
the in-car Auto vs Manual HVAC toggle, and `manual_override` is a low-SOC
override (same pattern as preconditioning-max, bioweapon, and climate
keeper), not Auto vs Manual mode.

Tesla has not published a VehicleAction for Auto vs Manual HVAC, or for
heater-off / vent-only as requested in
[issue #112](https://github.com/teslamotors/vehicle-command/issues/112).
Guessing an unused oneof number would collide with firmware this SDK
cannot update. Callers that ask for that mode get
[`protocol.ErrHvacAutoModeNotInProtocol`](error.go) from
`tesla-control hvac-auto-mode` and proxy paths `hvac_auto_mode` /
`set_hvac_auto` / `climate_manual` / `hvac_manual` / `auto_hvac_mode`
(HTTP 400 before a session). See
[issue #283](https://github.com/teslamotors/vehicle-command/issues/283).

Climate setpoints use `HvacTemperatureAdjustmentAction`. Firmware applies
`driver_temp_celsius` and `passenger_temp_celsius`. Proto3 leaves unset
floats at 0, which the vehicle treats as LO. Sending only
`absolute_celsius` (or a temperature zone) without those fields therefore
drops the setpoint to LO. `level` `TEMP_MIN` / `TEMP_MAX` are LO / HI,
not a flag that numeric temps are present. `Vehicle.ChangeClimateTemp`
and proxy `set_temps` encode driver and passenger Celsius and omit
`absolute_celsius` and `level`. `set_temps` requires at least one of
`driver_temp` / `passenger_temp` and copies a single value to both
seats; an empty body is not encoded as 0 °C.

`ClimateState.is_auto_conditioning_on` / `hvac_auto_request` are
**state**, not a command to switch Auto vs Manual.

Tesla has not published a VehicleAction for the in-car climate
**split / SYNC** control (linked vs independent driver and passenger
HVAC), and `ClimateState` has no `is_climate_split` / `is_sync` boolean
([issue #386](https://github.com/teslamotors/vehicle-command/issues/386)).
Independent setpoints over BLE are already
`HvacTemperatureAdjustmentAction.driver_temp_celsius` /
`passenger_temp_celsius` (`tesla-control climate-set-temp`, proxy
`set_temps`). `GetClimateState` returns `driver_temp_setting` and
`passenger_temp_setting`; unequal values are not the UI SYNC flag.
Guessing an unused oneof number, or adding a ClimateState field Tesla
has not shipped, would collide with firmware. Callers that ask for
that toggle get [`protocol.ErrClimateSplitNotInProtocol`](error.go)
from `tesla-control climate-split` and proxy paths `climate_split` /
`set_climate_split` / `climate_sync` / `set_climate_sync` (HTTP 400
before a session).

### Climate keeper Dog/Camp vs Child Presence Detection

`set_climate_keeper_mode` / `Vehicle.SetClimateKeeperMode` is published
(`HvacClimateKeeperAction`, VehicleAction field 44). Mode 2 is Dog,
mode 3 is Camp. tesla-http-proxy already maps the Fleet REST path.
HTTP 200 with `result: false` and reason `cpd_enabled` is an
application-layer [`NominalError`](error.go) from the car, not a missing
handler.

`cpd` is **Child Presence Detection** (occupancy / cabin radar firmware).
It is not the in-car **Child Left Alone Detection** setting. Turning that
UX toggle off does not clear `cpd_enabled`. Occupants (including adults)
can still trip CPD. `HvacClimateKeeperAction.manual_override` is a
low-SOC override, the same pattern as climate power; it is **not** a CPD
bypass. Tesla firmware still refuses Dog/Camp with `cpd_enabled` when
`manual_override` is true
([issue #437](https://github.com/teslamotors/vehicle-command/issues/437)).
A Tesla collaborator noted that exposing a CPD override in a public API
would require care.

Tesla has not published a `VehicleAction` to disable CPD, a confirmation
flow, or an extra OAuth scope for third-party Dog/Camp. Inventing an
unused oneof would collide with firmware. This SDK still delivers the
published action so a future firmware grant works without an SDK change.
Callers that ask this library to bypass CPD, or to treat Child Left Alone
Detection as sufficient, get
[`protocol.ErrClimateKeeperCPDFirmware`](error.go) from
`tesla-control climate-keeper-cpd` and proxy paths `climate_keeper_cpd`,
`override_cpd`, `dog_mode_cpd`, and `camp_mode_cpd` (HTTP 400 before a
session). Live vehicle refusals stay `NominalError` so tesla-http-proxy
keeps HTTP 200 `result:false`.
[`protocol.IsClimateKeeperCPDEnabled`](error.go) matches both. See
[issue #509](https://github.com/teslamotors/vehicle-command/issues/509).

### Charging Manager vs charge port

Enroll BLE charging gadgets as `ROLE_CHARGING_MANAGER` (not Owner). Owner
can unlock and remote-start; a charge-door automator should not. Charging
Manager already authorizes charging start/stop/amps. Opening the charge
port is a separate Infotainment action whose ACL is firmware. Until Tesla
expands it, the vehicle returns insufficient privileges; this client still
sends `ChargePortDoorOpen` so a future firmware grant works without an SDK
change. See [issue #413](https://github.com/teslamotors/vehicle-command/issues/413).

### Metadata serialization

The protocol requires peers to authenticate messages in a way that binds them
to associated metadata, such as the VIN. This in turn requires a canonical
method of serializing the metadata so it can be used as an input to a hash
function.

A metadata key/value pair is serialized using a tag-length-value encoding. 
Each metadata value has an associated numeric tag defined in
[Signatures.Tag](protobuf/signatures.proto). For example, the tag for a VIN is
`TAG_PERSONALIZATION = 2`. So the key-value pair `VIN: "abc"` would be serialized
as:

```
TLV(VIN: "abc") = TAG_PERSONALIZATION || LEN("abc") || "abc"
                      = 0x02 || 0x03 || 0x61 0x62 0x63
                      = 0x0203616263
```

Integer values are encoded as big-endian four-byte values:

```
TLV(COUNTER: 100) = TAG_COUNTER || LEN(uint32) || 0x00000064
                  = 0x05 || 0x04 || 0x00000064
                  = 0x050400000064
```

To serialize a set of metadata items, sort by tag in ascending order, serialize
each item, concatenate the results, and append a final `0xFF` byte to mark the
end of the metadata string:

```
SERIALIZE({COUNTER: 100, VIN: "abc"})
    = TLV(VIN: "abc") || TLV(COUNTER: 100) || 0xFF
    = 0x0203616263 || 0x050400000064 || 0xFF
    = 0x0203616263050400000064FF
```

## Notation

 * `c` - Client Private Key
 * `C = (Cx, Cy)` - Client Public key, a point on NIST-P256.
 * `v` - Vehicle Private key
 * `V = (Vx, Vy)` - Vehicle Public key, a point on NIST-P256.
 * `s[:n]` - The first n bytes of the string s
 * `BIG_ENDIAN(m, n)` - The n-byte big-endian encoding of m, padded with 0x00 bytes.
 * `K` - 128-bit AES-GCM key shared between the client and the vehicle
 * `ENCODE_PUBLIC(P)` - The encoding of a public key `P = (x, y)` as `0x04 ||
   BIG_ENDIAN(x, 32) || BIG_ENDIAN(y, 32)`; libraries refer to this as an
   uncompressed curve point or encoding.

## Test keys

This section contains example keys that will be used to generate test vectors
in the remainder of the document.


### Vehicle key

*Do not use this key except to debug implementations using the test vectors
included in these documents. Since the private key is published, enrolling the
public key on a vehicle may result in unauthorized access.*

In `vehicle.key`:

```
-----BEGIN EC PRIVATE KEY-----
MHcCAQEEIDRO5bRmp88e6xK29QMx2y5exYNO9fS+/P2MvlXCUo1woAoGCCqGSM49
AwEHoUQDQgAEx6H0cThIaqRymXFJSHjTOxok45Vx90im4WxZVbPYd9OmqqDpVRZk
dK9dMsQQ9DmiI0E3rRuwhf1OiBPJWPEdlw==
-----END EC PRIVATE KEY-----
```

In `vehicle.pem`:

```
-----BEGIN PUBLIC KEY-----
MFkwEwYHKoZIzj0CAQYIKoZIzj0DAQcDQgAEx6H0cThIaqRymXFJSHjTOxok45Vx
90im4WxZVbPYd9OmqqDpVRZkdK9dMsQQ9DmiI0E3rRuwhf1OiBPJWPEdlw==
-----END PUBLIC KEY-----
```

```
v  = 0x344EE5B466A7CF1EEB12B6F50331DB2E5EC5834EF5F4BEFCFD8CBE55C2528D70
Vx = 0xC7A1F47138486AA4729971494878D33B1A24E39571F748A6E16C5955B3D877D3
Vy = 0xA6AAA0E955166474AF5D32C410F439A2234137AD1BB085FD4E8813C958F11D97
HEX(ENCODE_PUBLIC(V)) = 04c7a1f47138486aa4729971494878d33b1a24e39571f748a6e16c5955b3d877d3a6aaa0e955166474af5d32c410f439a2234137ad1bb085fd4e8813c958f11d97
```

### Client key

*Do not use this key except to debug implementations using the test vectors
included in these documents. Since the private key is published, enrolling the
public key on a vehicle may result in unauthorized access.*

In `client.key`:

```
-----BEGIN EC PRIVATE KEY-----
MHcCAQEEICU4zcKal8GcHpmmN9bPT4yXDBGLVu3h5jI+bRYsSzDboAoGCCqGSM49
AwEHoUQDQgAEsra8aMLaBmXOZWgVWUmWxiOU7di+qQX+eBp1T+aoRacUMwkC8iXp
Jp1GbgWzSZgf2p2FzCPG+0RKpztikQXcbg==
-----END EC PRIVATE KEY-----
```

In `client.pem`:

```
-----BEGIN PUBLIC KEY-----
MFkwEwYHKoZIzj0CAQYIKoZIzj0DAQcDQgAEsra8aMLaBmXOZWgVWUmWxiOU7di+
qQX+eBp1T+aoRacUMwkC8iXpJp1GbgWzSZgf2p2FzCPG+0RKpztikQXcbg==
-----END PUBLIC KEY-----
```

```
c  = 0x2538CDC29A97C19C1E99A637D6CF4F8C970C118B56EDE1E6323E6D162C4B30DB
Cx = 0xB2B6BC68C2DA0665CE656815594996C62394EDD8BEA905FE781A754FE6A845A7
Cy = 0x14330902F225E9269D466E05B349981FDA9D85CC23C6FB444AA73B629105DC6E
HEX(ENCODE_PUBLIC(C)) = 04b2b6bc68c2da0665ce656815594996c62394edd8bea905fe781a754fe6a845a714330902f225e9269d466e05b349981fda9d85cc23c6fb444aa73b629105dc6e
```

## Handshake

This section assumes the client public key C is already enrolled in the
vehicle. Keys are bootstrapped by using the protocol or an NFC card to
authorize new keys using existing keys, with a Tesla Service key as the root
of trust. See the [README](/README.md) file in the repository root for
instructions on pairing a new key.

### Request

The client sends a RoutableMessage to the vehicle Domain it wishes to send
commands to, setting the `RoutableMessage.session_info_request.public_key` to
`ENCODE_PUBLIC(C)`.

Example handshake request for the [test client public key](#client-key):

```proto
to_destination {
  domain: DOMAIN_INFOTAINMENT
}
from_destination {
  routing_address: 2c907bd76c640d360b3027dc7404efde
}
session_info_request {
  public_key: 04b2b6bc68c2da0665ce656815594996c62394edd8bea905fe781a754fe6a845a714330902f225e9269d466e05b349981fda9d85cc23c6fb444aa73b629105dc6e
}
uuid: 1588d5a30eabc6f8fc9a951b11f6fd11
```

### Response

The vehicle response has the `RoutableMessage.session_info` populated. The
client parses this field as a `SessionInfo` message (defined in
[signatures.proto](protobuf/signatures.proto)).

The `RoutableMessage.signature_data` contains information that can be used to
verify the integrity (but not authenticity) of the response, as discussed
later.

Example response:

```proto
to_destination {
  routing_address: 2c907bd76c640d360b3027dc7404efde
}
from_destination {
  domain: DOMAIN_INFOTAINMENT
}
signature_data {
  session_info_tag {
    tag: 996c1fe38331be138f8039c194b14db2198846ed7d8251e6749284d7b32ea002
  }
}
session_info: 0806124104c7a1f47138486aa4729971494878d33b1a24e39571f748a6e16c5955b3d877d3a6aaa0e955166474af5d32c410f439a2234137ad1bb085fd4e8813c958f11d971a104c463f9cc0d3d26906e982ed224adde6255a0a0000
request_uuid: 1588d5a30eabc6f8fc9a951b11f6fd11
```

The `session_info` field decodes as:
```
counter: 6
publicKey: 04c7a1f47138486aa4729971494878d33b1a24e39571f748a6e16c5955b3d877d3a6aaa0e955166474af5d32c410f439a2234137ad1bb085fd4e8813c958f11d97
epoch: 4c463f9cc0d3d26906e982ed224adde6
clock_time: 2650
```

The session info fields are used to authorize commands.

### Key Agreement

The client and the vehicle derive a shared 128-bit AES-GCM key K using ECDH:

```
S = (Sx, Sy) = ECDH(c, V) = ECDH(v, C)
K = SHA1(BIG_ENDIAN(Sx, 32))[:16]
```

---

***Example:*** Computing K using `client.key` and `vehicle.pem` from above with
OpenSSL:

```bash
export K=$(openssl pkeyutl -derive -inkey client.key -peerkey vehicle.pem \
    | openssl dgst -sha1 -binary \
    | head -c 16 \
    | xxd -p)
echo $K
1b2fce19967b79db696f909cff89ea9a
```

---

Use a mature cryptographic library such as OpenSSL to compute K. A custom
implementation will likely be vulnerable to subtle attacks.

The protocol always uses the binary encoding of K; the hex representation above
is only for printability.

### Response authentication

In order to prevent a man-in-the-middle (MITM) attacker from obtaining a
command that expires later than intended, the client must authenticate the
session info response. The vehicle will reject relayed commands if the attacker
modifies the included public key, and the client will reject a modified
response if attacker leaves the public key intact.

The client first derives a session info authentication key from the shared
secret K:

```
SESSION_INFO_KEY = HMAC-SHA256(K, "session info")
```

The "session info" is the literal ASCII-encoded string.

---

***Example:*** Using the test vector value for `K` above as `hexkey` below,

```bash
export SESSION_INFO_KEY=$(\
    echo -n "session info" \
    | openssl dgst -sha256 -mac hmac -macopt hexkey:"$K")
echo $SESSION_INFO_KEY
fceb679ee7bca756fcd441bf238bf2f338629b41d9eb9c67be1b32c9672ce300
```

---

Next, the client [serializes the following metadata](#Metadata-serialization):

 * `TAG_SIGNATURE_TYPE`: `Signatures.SIGNATURE_TYPE_HMAC`
 * `TAG_PERSONALIZATION`: `VIN`
 * `TAG_CHALLENGE`: UUID from session info request message

---

***Example:*** Continuing with the above example values and `VIN =
5YJ30123456789ABC`, the serialized metadata is

```
METADATA = TLV(TAG_SIGNATURE_TYPE, Signatures.SIGNATURE_TYPE_HMAC) ||
           TLV(TAG_PERSONALIZATION, "5YJ30123456789ABC") ||
           TLV(TAG_CHALLENGE, 1588d5a30eabc6f8fc9a951b11f6fd11) ||
           ff
         = 00 01 06 || // Tag, length, value
           02 11 35594a3330313233343536373839414243 || // etc.
           06 10 1588d5a30eabc6f8fc9a951b11f6fd11 |\
           ff
         = 000106021135594a333031323334353637383941424306101588d5a30eabc6f8fc9a951b11f6fd11ff
```

The client computes the expected HMAC-SHA256 tag using the serialized metadata
and session info.

```bash
export SESSION_INFO=0806124104c... (truncated from above)
echo "$METADATA$SESSION_INFO" | xxd -r -p | openssl dgst -sha256 -mac hmac -macopt hexkey:"$SESSION_INFO_KEY"
996c1fe38331be138f8039c194b14db2198846ed7d8251e6749284d7b32ea002
```

---

The client compares this value to the
`RoutableMessage.signature_data.session_info_tag.tag` from the vehicle
response.

*Warning:* Always use a constant-time comparison function when validating HMAC
tags. All mature cryptographic libraries will have a special function
documented for this purpose.

The client must discard responses with an invalid tag.

## Authorizing commands

After completing the above handshake process, the client has:

 * The vehicle [time](#Time), expressed as an `(epoch, timestamp)` pair.
 * The anti-replay counter
 * The shared key symmetric key `K`.

The client first generates a command protobuf `P`, which generally encodes
either a `VCSEC.UnsignedMessage` or a `CarServer.Action` message. To determine
the required message type and contents for a given command, follow source code
links from the [package
documentation](https://pkg.go.dev/github.com/teslamotors/vehicle-command/pkg/vehicle).

### Metadata

The client serializes the following metadata values into a string `M`:

| Value | Tag | Description |
| ----- | --- | ----------- |
| Signature type | `Signatures.TAG_SIGNATURE_TYPE` | Either `Signatures.SIGNATURE_TYPE_HMAC_PERSONALIZED` or `Signatures.SIGNATURE_TYPE_AES_GCM_PERSONALIZED`. See below. |
| Domain         | `Signatures.TAG_DOMAIN`         | Typically `UniversalMessage.DOMAIN_VEHICLE_SECURITY` or `UniversalMessage.DOMAIN_INFOTAINMENT` |
| VIN            | `Signatures.TAG_PERSONALIZATION`| 17-character vehicle identification number |
| Epoch          | `Signatures.TAG_EPOCH`          | Copied from `session_info.epoch`           |
| Expiration time| `Signatures.TAG_EXPIRES_AT`     | Time in seconds according to domain's clock |
| Counter        | `Signatures.TAG_COUNTER`        | Monotonic counter, initially `session_info.counter` |
| Flags          | `Signatures.TAG_FLAGS`          | Request flag bit mask. Only included if non-zero. |

Setting the expiration time requires the client to track the difference between
the domain's clock and the local clock. Note that each domain has its own clock.

The counter must increase with each command within the same epoch. Infotainment
tracks a sliding window of valid counters to allow for out-of-order message
arrival, while VCSEC requires that messages arrive in counter order.

### Authentication methods

Vehicles support two authentication methods: HMAC-SHA256 and AES-GCM. Messages
sent with HMAC-SHA256 authentication are sent in plaintext. This allows the
Fleet API backend to (1) reject commands based on OAuth scopes and (2) drop
deprecated messages sent by VCSEC that would require a more complex client
state machine to disambiguate. Commands sent with AES-GCM are encrypted; Fleet
API blocks these commands because it cannot enforce OAuth scopes.

The official Golang package uses AES-GCM over BLE and HMAC-SHA256 over Fleet
API.

### HMAC-SHA256 authentication

To add HMAC-SHA256 authentication:

 - Derive an HMAC-SHA256 key `K' = HMAC-SHA256(K, "authenticated command")`.
   The "authenticated command" is a string literal.
 - Construct a RoutableMessage as described [above](#Message-encoding).
 - Set the `RoutableMessage.payload.protobuf_message_as_bytes` to `P`.
 - Set the `RoutableMessage.signature_data.signer_identity.public_key` to
   `ENCODE_PUBLIC(C)` (see [Notation](#Notation)).
 - Compute the HMAC tag `tag = HMAC-SHA256(K', M || P)`.
 - Populate `RoutableMessage.signature_data.HMAC_PersonalizedData` with the
   metadata values and authentication `tag`.

### AES-GCM encryption

To encrypt the plaintext protobuf `P` using AES-GCM:

 - Set the 128-bit encryption key to `K`
 - Use a random 12-byte nonce (sometimes called an initialization vector, or
   IV). Some libraries may choose one for you.
 - Set the Associated Authenticated Data (AAD) field to `SHA256(M)`
 - Encrypt the `P` with the above parameters to obtain a ciphertext `x` and a
   message authentication `tag`.
 - Construct a RoutableMessage as described [above](#Message-encoding).
 - Set the `RoutableMessage.payload.protobuf_message_as_bytes` to `x`.
 - Set the `RoutableMessage.signature_data.signer_identity.public_key` to
   `ENCODE_PUBLIC(C)` (see [Notation](#Notation)).
 - Populate
   `RoutableMessage.signature_data.AES_GCM_Personalized_Signature_Data` with
   the metadata values, authentication `tag`, and nonce.

---

***Example:*** We'll send an "Turn HVAC on" command using the above example
with `VIN = 5YJ30123456789ABC` and the hex representation of the shared key `K
= 1b2fce19967b79db696f909cff89ea9a`. The request sets the
`UniversalMessage.FLAG_ENCRYPT_RESPONSE` flag, as recommended for all commands
(see [Response decryption](#response-decryption)), so the flags bit mask is `1
<< FLAG_ENCRYPT_RESPONSE = 2` and is included in both the metadata and the
`RoutableMessage.flags` field.

First we find the protobuf encoding for the command. Normally this is done
using the language-specific bindings created by `protoc` from the `*.proto`
files, but for purposes of illustration we'll do it from the command line:

```bash
echo 'vehicleAction {
  hvacAutoAction {
    power_on: true
  }
}' | protoc --encode=CarServer.Action -I protobuf protobuf/*.proto | xxd -p
```

Output: `120452020801`.

Next, we construct the serialized metadata string from values in the table
below. The metadata string is used as the associated authenticated data (AAD)
field of AES-GCM.

| Tag   | Value | Encoding |
| ----- | ----- | -------- |
|  `Signatures.TAG_SIGNATURE_TYPE` | `Signatures.SIGNATURE_TYPE_AES_GCM_PERSONALIZED = 0x05` | 00 01 05 |
|  `Signatures.TAG_DOMAIN`         | `UniversalMessage.DOMAIN_INFOTAINMENT = 0x03`           | 01 01 03 |
|  `Signatures.TAG_PERSONALIZATION`| `5YJ30123456789ABC`    | 02 11 35594a3330313233343536373839414243  |
|  `Signatures.TAG_EPOCH`          | `session_info.epoch`   | 03 10 4c463f9cc0d3d26906e982ed224adde6    |
|  `Signatures.TAG_EXPIRES_AT`     | `t=2655` seconds       | 04 04 00000a5f                            |
|  `Signatures.TAG_COUNTER`        | `session_info.counter` | 05 04 00000007                            |
|  `Signatures.TAG_FLAGS`          | `1 << UniversalMessage.FLAG_ENCRYPT_RESPONSE = 0x02` | 07 04 00000002 |

All values in the Encoding column are hex, including the leading tag and
length bytes.

Concatenating the above encoded values together with the terminal `0xff` byte gives:

```
000105010103021135594a333031323334353637383941424303104c463f9cc0d3d26906e982ed224adde6040400000a5f050400000007070400000002ff
```

```python
import os
from cryptography.hazmat.primitives.ciphers.aead import AESGCM
from cryptography.hazmat.primitives import hashes

plaintext = bytes.fromhex("120452020801")

metadata = bytes.fromhex("000105010103021135594a333031323334353637383941424303104c463f9cc0d3d26906e982ed224adde6040400000a5f050400000007070400000002ff")
aad = hashes.Hash(hashes.SHA256())

aad.update(metadata)
key = bytes.fromhex("1b2fce19967b79db696f909cff89ea9a")
aesgcm = AESGCM(key)

nonce = os.urandom(12)
ct = aesgcm.encrypt(nonce, plaintext, aad.finalize())
print(f"Nonce: {nonce.hex()}, Ciphertext: {ct[:-16].hex()}, Tag: {ct[-16:].hex()}")
```

Output (will be randomized each time by the nonce):

```
Nonce: dbf79447fa156674dae1caed, Ciphertext: 38038e8c0f2e, Tag: c228e0ff64991481db3a7bbc133696c5
```

Copying the above fields into a RoutableMessage protobuf yields:

```protoc
to_destination {
  domain: DOMAIN_INFOTAINMENT
}
from_destination {
  routing_address: 2c907bd76c640d360b3027dc7404efde
}
protobuf_message_as_bytes: 38038e8c0f2e
signature_data {
  signer_identity {
    public_key: 04b2b6bc68c2da0665ce656815594996c62394edd8bea905fe781a754fe6a845a714330902f225e9269d466e05b349981fda9d85cc23c6fb444aa73b629105dc6e
  }
  AES_GCM_Personalized_data {
    epoch: 4c463f9cc0d3d26906e982ed224adde6
    nonce: dbf79447fa156674dae1caed
    counter: 7
    expires_at: 2655
    tag: c228e0ff64991481db3a7bbc133696c5
  }
}
flags: 2
uuid: 58406580528b6a5301391800b4fe9b99
```

The sender may omit the `epoch` field from the protobuf, but including the
epoch allows the vehicle to return a more specific error message if there is a
mismatch.

---

### Caching session state

If a client is not running continuously, it should cache session state to disk,
along with the time difference between the local clock and the vehicle clock.
Loading the session from cache removes the need to send session info requests,
which reduces the latency of the first command and, when using Fleet API,
reduces the number of Fleet API requests made by the client. If the session
state is no longer valid, then the client can automatically recover as
described below. The recovery mechanism is no more expensive than performing
the handshake in the first place, so clients never incur a penalty by
optimistically assuming a cache is valid.

### Recovering from synchronization errors

The vehicle may include up-to-date session state in an error message in cases
where an authentication error could be attributed to a synchronization fault.
For example, if the infotainment system reboots, then the vehicle and the
client may not be using the same epoch.

The client MUST discard the session information if any of the following are
true:

 - The client did not use the request UUID in the last several seconds.
 - The session info HMAC tag is incorrect.
 - The clock time is earlier than the clock time in a previously authenticated
   session info message with the same epoch.

The client MUST update its session state if none of the above are true. When
updating its session state, the client MUST NOT rollback its anti-replay counter
unless the epoch changes. These are not security requirements per se, since the
vehicle bears responsibility for rejecting replayed messages; however, this
behavior allows clients to more reliably handle poor connectivity and
out-of-order message delivery.

## Response handling

Note that if client transmits a command and does not receive a response back
from the vehicle, the vehicle may still have received and executed the command.
This is especially likely if the vehicle or client is on an unreliable network.

### Protocol-layer errors

Vehicle responses also RoutableMessages. If a protocol-layer error occurred,
then the vehicle sets the
`RoutableMessage.signedMessageStatus.signed_message_fault` field.

Error codes and their remediation are summarized in
[universal_message.proto](protobuf/universal_message.proto).
See comments in the `MessageFault_E` definition.

#### Response size limits

Vehicles enforce a fixed upper bound on the size of a serialized response
`RoutableMessage`. If a reply would exceed that bound, the vehicle discards it
and instead returns `MESSAGEFAULT_ERROR_RESPONSE_MTU_EXCEEDED` with no payload.
Note the following:

 * The request *was* received and processed. For commands, this means the
   command may have executed even though the client did not receive a
   confirmation.
 * The bound is a vehicle-side memory budget, not the negotiated transport MTU.
   Renegotiating the BLE ATT MTU or reconnecting does not raise it. Clients
   have observed the limit at roughly 450 bytes for the complete serialized
   response.
 * Retransmitting the same request produces the same oversized reply, so the
   error is not treated as transient.
 * The size of the reply is determined by the vehicle. Most `GetVehicleData`
   requests are empty marker messages with no field selection, so a client
   cannot request a smaller response. A known trigger is `GetDriveState` while
   a navigation route with a long destination name is active; the failure
   clears once the route ends. See
   [issue #472](https://github.com/teslamotors/vehicle-command/issues/472).

### Response decryption

If the client set the `FLAG_ENCRYPT_RESPONSE`, then vehicles running supported
firmware (2024.38+) will encrypt the payload. Vehicles running previous
firmware versions will ignore this flag, so clients do not need to check the
vehicle's firmware version before setting it.

If the response includes a `signature_data.AES_GCM_Response_data` field, then
the `protobuf_message_as_bytes` payload is encrypted. Otherwise, the payload is
plaintext.

#### Request hash

The client must compute the "request hash" of each request it sends. This value
will be used when decrypting the response. The request hash is a single byte
that encodes the authentication method used by the request (either
`Signatures.SIGNATURE_TYPE_HMAC_PERSONALIZED` or
`Signatures.SIGNATURE_TYPE_AES_GCM_PERSONALIZED`) followed by the request's
authentication tag.

 * If the request uses AES-GCM encryption, then the tag is the request's
   `signature_data.AES_GCM_Personalized_Signature_Data.tag` field.
 * If the request uses HMAC-SHA256 authentication, then the tag is the
   request's `signature_data.HMAC_PersonalizedData.tag` field.

If the request is sent to the Vehicle Security domain, then the request hash is
truncated to 17 bytes (i.e., a single byte encoding the authentication method
and the first 16 bytes of the MAC). Truncation only affects HMAC-SHA256
requests, since AES-GCM tags are only 16 bytes.

#### Counter verification

The client must verify that the counter value contained in the response has
not been previously used in a response for the same request. Note that if a
request does trigger multiple responses, responses may be received out of
order.

Clients that fail to implement this check are vulnerable to replay attacks.

#### Response metadata

To decrypt the response, use AES-GCM with the shared key `K` defined above, the
nonce and tag supplied in the response, and the associated authenticated data
field obtained by [serializing](#Metadata serialization) the following metadata:

| Value | Tag | Description |
| ----- | --- | ----------- |
| Signature type | `Signatures.TAG_SIGNATURE_TYPE` | `Signatures.SIGNATURE_TYPE_AES_GCM_RESPONSE` |
| Domain         | `Signatures.TAG_DOMAIN`         | Typically `UniversalMessage.DOMAIN_VEHICLE_SECURITY` or `UniversalMessage.DOMAIN_INFOTAINMENT` |
| VIN            | `Signatures.TAG_PERSONALIZATION`| 17-character vehicle identification number |
| Counter        | `Signatures.TAG_COUNTER`        | Monotonic counter, initially `session_info.counter` |
| Flags          | `Signatures.TAG_FLAGS`          | Flags field of the response, not the request |
| Request Hash   | `Signatures.TAG_REQUEST_HASH`   | See [Request hash](#request-hash) |
| Fault          | `Signatures.TAG_FAULT`          | Message fault of the response |

Note that the Flags field is always included in the response metadata, whereas
for outgoing requests the flags are only included if non-zero. Encode the fault
as a 32-bit big-endian integer.

### Infotainment application-layer responses

If a reply comes from the Infotainment domain, the client should parse the
`protobuf_message_as_bytes` field as a `CarServer.Response`, defined in
[car_server.proto](protobuf/car_server.proto). An application-layer status code
is set in `Response.actionStatus`.

### VCSEC application-layer responses

If a reply comes from the Vehicle Security (VCSEC) domain, the client should
parse the `protobuf_message_as_bytes` field as a `VCSEC.FromVCSECMessage`,
defined in [vcsec.proto](protobuf/vcsec.proto). VCSEC emits up to three
responses to a given request. Clients that use Fleet API only receive the
final response; clients that use BLE will need to implement special logic to
determine what responses are final, as described below.

If the client sent a request to pair a new key using the NFC card, then VCSEC
sends `FromVSCEC.commandStatus.operationStatus = OPERATIONSTATUS_WAIT` to
indicate it is waiting for the NFC card tap.

For other requests, `OPERATIONSTATUS_WAIT` indicates VSCEC is busy with some
other request and the client should retry after a short delay.

Other values of `FromVCSEC.commandStatus.operationStatus` can be ignored. In
particular, a value of `OPERATIONSTATUS_ERROR` is sent only for the benefit of
legacy clients. New clients should discard the message and wait for a more
specific error code in a subsequent message.

If the client sent a whitelist operation request (e.g., add or remove another
key), a message is terminal if
`FromVCSECMessage.commandStatus.whitelistOperationStatus` is populated. The
client should drop empty messages.

For non-whitelist operations, an empty message indicates success and
`FromVCSECMessage.nominalError` indicates an error.
