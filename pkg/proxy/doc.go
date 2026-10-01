/*
Package proxy implements a REST API for sending commands to Tesla vehicles.

See the [Fleet API documentation] for available endpoints.

# Tent mode and suspension

POST /api/1/vehicles/{vin}/command/set_tent_mode with {"on": true} enables
Cybertruck tent mode (VehicleAction field 94). The vehicle typically refuses
unless it is in Park.

POST set_suspension_level with {"suspension_level": "medium"} (string names
or 1–6) sets air-suspension ride height (VehicleAction field 118). "level"
is an alias for medium. POST level_suspension with an empty body is the same
as medium. Prefer set_tent_mode when the bed should auto-level each corner.

See teslamotors/vehicle-command#424.

# WiFi configuration

POST wifi_on, wifi_off, set_wifi, add_wifi_network, forget_wifi_network, or
wifi_connect_in_drive returns HTTP 400 with
[protocol.ErrWiFiNotInProtocol] before opening a vehicle session. Tesla has
not published signed VehicleAction fields for those in-car settings; this
proxy will not invent them or forward a PSK. See
teslamotors/vehicle-command#419. WiFi/cellular *state* streaming is
teslamotors/fleet-telemetry#407.

# Keeping infotainment awake

POST keep_awake or keep_alive returns HTTP 400 with
[protocol.ErrKeepAwakeNotInProtocol] before opening a vehicle session.
wake starts infotainment but does not inhibit sleep.
keep_accessory_power_mode powers the 12V jack and charging USB ports, not
the glovebox dashcam/data USB. Tesla has not published a keep-alive
VehicleAction. See teslamotors/vehicle-command#397.

# Charging while Infotainment is asleep

POST charge_stop and set_charging_amps still send the published
Infotainment VehicleActions. Fleet Telemetry can report ACChargingPower
and Soc while Tesla signed_command returns vehicle unavailable (offline
or asleep); inet.ErrVehicleNotAwake maps to HTTP 408. wake starts
infotainment but does not inhibit sleep. POST charging_while_asleep,
charge_stop_asleep, or set_charging_amps_asleep returns HTTP 400 with
[protocol.ErrChargingWhileInfotainmentAsleep] before opening a vehicle
session. This proxy will not invent keep-awake or a charging-controller
bypass. See teslamotors/vehicle-command#452.

# Infotainment signed_command while the vehicle list says online

POST honk_horn and other published Infotainment commands are still sent.
GET /api/1/vehicles state=online and a successful VCSEC signed_command do
not prove DOMAIN_INFOTAINMENT will be accepted. Tesla's gateway can return
HTTP 408 vehicle is offline for that domain (inet.ErrVehicleNotAwake),
including on fleet-api.prd.cn while the car is occupied and BLE works.
POST infotainment_offline, domain_infotainment_online, or
vcsec_means_infotainment_online returns HTTP 400 with
[protocol.ErrInfotainmentSignedCommandOffline] before opening a vehicle
session. This proxy will not rewrite the domain to VCSEC or skip the
Infotainment handshake. See teslamotors/vehicle-command#285.

# Battery option codes

POST battery_size, get_battery_option, or get_battery_size returns HTTP 400
with [protocol.ErrBatteryOptionRequiresFleetAPI] before opening a vehicle
session. Pack identity is Tesla catalog metadata
(GET /api/1/dx/vehicles/options), not a signed command. Tesla often omits
$BT* codes; this proxy will not invent them. See
teslamotors/vehicle-command#391.

# BLE key presence / Walk-Away Door Lock

POST ble_presence_exempt or command_only_key returns HTTP 400 with
[protocol.ErrBLEKeyPresenceNotInProtocol] before opening a vehicle session.
add-key-request enrolls a VCSEC whitelist key. Tesla has not published a
flag that lets an in-car BLE client authorize commands while being ignored
for Walk-Away Door Lock. Disconnect after each command. See
teslamotors/vehicle-command#480.

# Scheduled charging / cabin overheat

POST scheduled_charging_overheat or force_scheduled_charging returns HTTP
400 with [protocol.ErrScheduledChargingFirmware] before opening a vehicle
session. set_scheduled_charging and set_scheduled_departure still deliver
the published schedule commands. This proxy will not disable cabin
overheat protection as a workaround for Intel-MCU Model S firmware that
does not sleep/wake the scheduler. See teslamotors/vehicle-command#342.

# Remote boombox

POST remote_boombox returns HTTP 400 with
[protocol.ErrBoomboxNotInProtocol] before opening a vehicle session.
Fleet API still lists the path; Tesla has not published a VehicleAction
for the external speaker pending legal review of Pedestrian Warning
System restrictions (teslamotors/vehicle-command#266). honk_horn and
flash_lights are published. This proxy will not invent a field number,
copy a third-party firmware dump, or map boombox onto honk. See
teslamotors/vehicle-command#266 and #411.

# HVAC Auto vs climate power

POST auto_conditioning_start / auto_conditioning_stop map to
Vehicle.SetClimatePower / ClimateOff (HvacAutoAction.power_on).
That is climate power, not Auto vs Manual HVAC. Optional
manual_override on auto_conditioning_start is the published low-SOC
override bit. POST hvac_auto_mode, set_hvac_auto, climate_manual,
hvac_manual, or auto_hvac_mode returns HTTP 400 with
[protocol.ErrHvacAutoModeNotInProtocol] before opening a vehicle
session. Tesla has not published a VehicleAction for Auto vs Manual
HVAC. set_temps requires driver_temp and/or passenger_temp
(ChangeClimateTemp); an empty body is not encoded as 0 °C (LO). See
teslamotors/vehicle-command#283.

# Climate split / SYNC

POST climate_split, set_climate_split, climate_sync, or
set_climate_sync returns HTTP 400 with
[protocol.ErrClimateSplitNotInProtocol] before opening a vehicle
session. Tesla has not published a VehicleAction for the in-car
split/SYNC control. Independent driver and passenger setpoints are
already set_temps (HvacTemperatureAdjustmentAction). GetClimateState
returns the two temp settings, not a split boolean. See
teslamotors/vehicle-command#386.

# Virtual key Finish Setup return URL

POST virtual_key_return, ak_return_uri, or set_virtual_key_return returns
HTTP 400 with [protocol.ErrVirtualKeyReturnURI] before opening a vehicle
session. https://tesla.com/_ak/<domain> is Tesla's hosted enrollment page.
This proxy does not append return_uri. An off-domain return URL is an
open redirect; Tesla said any redirect must stay on the registered
partner domain and be configured in advance. See
teslamotors/vehicle-command#444.

# Climate keeper Dog/Camp vs CPD

POST set_climate_keeper_mode still sends HvacClimateKeeperAction
(0=off, 1=on, 2=Dog, 3=Camp). Firmware may refuse with HTTP 200
result:false and reason cpd_enabled (Child Presence Detection
occupancy). That is not the Child Left Alone Detection setting.
manual_override is a low-SOC override, not a CPD bypass
(teslamotors/vehicle-command#437). POST climate_keeper_cpd,
override_cpd, dog_mode_cpd, or camp_mode_cpd returns HTTP 400 with
[protocol.ErrClimateKeeperCPDFirmware] before opening a vehicle
session. This proxy will not invent a CPD-disable VehicleAction.
See teslamotors/vehicle-command#509.

# Charging Manager vs charge port

POST charge_port_door_open / charge_port_door_close still send
ChargePortDoorOpen/Close. Role checks are firmware. Charging Manager
keys can charging-start/stop/set-amps; charge-port may return
INSUFFICIENT_PRIVILEGES until Tesla expands that ACL. POST
charging_manager_charge_port, grant_charging_manager_charge_port, or
charging_manager_port returns HTTP 400 with
[protocol.ErrChargingManagerChargePortFirmware] before opening a
vehicle session. This proxy will not enroll Owner or rewrite charge-port
as a VCSEC closure. See teslamotors/vehicle-command#413.

# BLE GetDriveState latency

POST ble_state_fast, drive_state_fast, or set_ble_poll_interval returns
HTTP 400 with [protocol.ErrBLEStateLatencyFirmware] before opening a
vehicle session. GetState is one Infotainment round-trip (~250–300ms
observed). This proxy cannot guarantee <150ms, disable response
encryption, or stream DriveState. Reuse StartSession; fleet-telemetry
is a separate product. See teslamotors/vehicle-command#414.

# Seat heater and cooler

POST remote_seat_heater_request and remote_seat_cooler_request already
map to published HvacSeatHeaterActions (field 36) and
HvacSeatCoolerActions (field 49). The heater body accepts Owner/Fleet
JSON "heater" as an alias for "seat_position" (0–8) plus "level"
(0–3). Cooler uses "seat_position" plus "seat_cooler_level" ("level"
is an alias).

HTTP 501 with JSON error Unauthorized from Tesla signed_command is
Fleet API partner/region/OAuth allowlist. writeJSONError forwards
Tesla's status, so clients see "Not Implemented" even though this
proxy already signed and POSTed the VehicleAction. That is not
[ErrCommandNotImplemented]. POST seat_heater_not_implemented,
seat_cooler_not_implemented, or remote_seat_climate_not_implemented
returns HTTP 400 with [protocol.ErrSeatClimateFleetAPI] before opening
a vehicle session. See teslamotors/vehicle-command#383.

# Partner OAuth audience / policy

POST partner_token, register_partner, oauth_audience, or
invalid_audience returns HTTP 400 with
[protocol.ErrPartnerOAuthNotProvisioned] before opening a vehicle
session. Tesla Fleet Auth invalid_audience on client_credentials and
/authorize "No policy rules" mean Tesla has not bound OAuth
policy/audience to the application. tesla-http-proxy consumes an
already-issued OAuth token; it does not mint partner tokens or POST
/api/1/partner_accounts. Cycling regional fleet-api audience URLs
does not provision Tesla's IdP. Use Tesla developer dashboard Support
Inquiry. See teslamotors/vehicle-command#460.

# Request cancellation

Per-request timeouts in forwardRequest and handleVehicleCommand are derived
from req.Context() so a client disconnect cancels pending Fleet API I/O and
vehicle lookup. Dispatcher.Start stops its listener if that context is
canceled before Disconnect would run. This does not recall a command that
has already been delivered to the vehicle. See teslamotors/vehicle-command#491.

[Fleet API documentation]: https://developer.tesla.com/docs/fleet-api/getting-started/what-is-fleet-api
*/
package proxy
