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
     [`carsever.Action`](protobuf/car_server.proto).
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
 * The `flags` field is a bit mask of `universal_message.Flags` values. There
   is currently no need for clients to use this field; future versions of the
   protocol may use this field to add authenticated data that is discarded by
   older vehicles.

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

A **Charging Manager** can read vehicle data and authorize commands that affect
vehicle charging.

A **Service** key bootstraps pairing other keys and authorizes commands on
behalf of service technicians. Service keys can remotely (un)lock vehicles in
order to provide roadside assistance, but vehicles in their default state
prevent Service keys from authorizing other commands over the Internet.

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
public key on a vehicle may result in unauthorized access.

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

Next, the client [serializes the following metadata](#Metadata serialization):

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
| Domain         | `Signatures.TAG_DOMAIN`         | Typically `UniversalMesasge.DOMAIN_VEHICLE_SECURITY` or `UniversalMessage.DOMAIN_INFOTAINMENT` |
| VIN            | `Signatures.TAG_PERSONALIZATION`| 17-character vehicle identification number |
| Epoch          | `Signatures.TAG_EPOCH`          | Copied from `session_info.epoch`           |
| Expiration time| `Signatures.TAG_EXPIRES_AT`     | Time in seconds according to domain's clock |
| Counter        | `Signatures.TAG_COUNTER`        | Monotonic counter, initially `session_info.counter` |

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
 - Set the `RoutableMessge.payload.protobuf_message_as_bytes` to `P`.
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
 - Set the `RoutableMessge.payload.protobuf_message_as_bytes` to `x`.
 - Set the `RoutableMessage.signature_data.signer_identity.public_key` to
   `ENCODE_PUBLIC(C)` (see [Notation](#Notation)).
 - Populate
   `RoutableMessage.signature_data.AES_GCM_Personalized_Signature_Data` with
   the metadata values, authentication `tag`, and nonce.

---

***Example:*** We'll send an "Turn HVAC on" command using the above example
with `VIN = 5YJ30123456789ABC` and the hex representation of the shared key `K
= 1b2fce19967b79db696f909cff89ea9a`.

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

Concatenating the above encoded values together with the terminal `0xff` byte gives:

```
000105010103021135594a333031323334353637383941424303104c463f9cc0d3d26906e982ed224adde6040400000a5f050400000007ff
```

```python
import os
from cryptography.hazmat.primitives.ciphers.aead import AESGCM
from cryptography.hazmat.primitives import hashes

plaintext = bytes.fromhex("120452020801")

metadata = bytes.fromhex("000105010103021135594a333031323334353637383941424303104c463f9cc0d3d26906e982ed224adde6040400000a5f050400000007ff")
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
Nonce: dbf79447fa156674dae1caed, Ciphertext: 38038e8c0f2e, Tag: 8e128da165f162f4d7d2c8da866cf82a
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
    tag: 8e128da165f162f4d7d2c8da866cf82a
  }
}
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
`FromVCSECMessage.commandStatus.nominalError` indicates an error.
