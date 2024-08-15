# Tesla JWS Utility

The `tesla-jws` utility can be used to generate JSON Web Signatures (JWSs) for
[fleet telemetry](https://github.com/teslamotors/fleet-telemetry)
configurations. These are distinct from the JSON Web Tokens (JWTs) [granted by
customers](https://developer.tesla.com/docs/fleet-api/authentication/third-party-tokens) to
third parties that allow limited vehicle or account access.

## Signing a fleet telemetry configuration

There are two methods of signing a fleet telemetry configuration. See below for
examples.

 - Use the `tesla-jws` command-line tool to generate the JWS, then POST the JWS
   to the Fleet API endpoint `/api/1/vehicles/fleet_telemetry_config_jws`.
 - OR use the `tesla-http-proxy` [tool](/cmd/tesla-http-proxy), and POST the
   unsigned configuration to the proxy endpoint
   `/api/1/vehicles/fleet_telemetry_config`. The proxy will sign the
   configuration and POST the resulting JWS to Fleet API.

Both of these are self-service alternatives to the `fleet_telemetry_config`
[Fleet API endpoint](https://developer.tesla.com/docs/fleet-api/endpoints/vehicle-endpoints#fleet-telemetry-config-create),
which requires Tesla to issue a certificate for your server.

### Using tesla-jws

If the fleet telemetry configuration file is in `telemetry_config.json`, run:

```bash
tesla-jws -fleet sign TelemetryClient telemetry_config.json > signed-config.jws
```

This invocation assumes you are using environment variables to specify your
private key. Alternatively, you can use `-key-name` or `-key-file`; see the
[repository README](/README.md#installation-and-configuration) for more
information.

**Note**: The `signed-config.jws` output file is not sensitive. It has the same
syntax as a JSON Web Token (JWT), but is not a bearer authorization token.

Next, follow the Fleet API
[instructions](https://developer.tesla.com/docs/fleet-api/endpoints/vehicle-endpoints#fleet-telemetry-config-jws)
for submitting `signed-config.jws`.

### Using tesla-http-proxy

The [HTTP proxy server](/README.md#using-the-http-proxy) implements the
[`fleet_telemetry_config`
endpoint](https://developer.tesla.com/docs/fleet-api/endpoints/vehicle-endpoints#fleet-telemetry-config-create).

```bash
curl \
    --cacert cert.pem \
    --header 'Content-Type: application/json' \
    --header "Authorization: Bearer $TESLA_AUTH_TOKEN" \
    -X POST \
    --data-binary "@telemetry.json" \
    "https://localhost:4443/api/1/vehicles/fleet_telemetry_config"
```
