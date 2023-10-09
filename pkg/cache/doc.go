// Package cache allows clients to resume authenticated sessions with a Tesla vehicle.
//
// When a client communicates with a vehicle for the first time, the protocol requires an extra
// round-trip to perform a handshake. Using a [SessionCache] allows the client to avoid that
// round-trip on subsequent connections. If the SessionCache is outdated (e.g., because the
// vehicle's security controller rebooted during a firmware update), then the first command sent by
// the client will fail, and the vehicle will respond with updated session information. This does
// not introduce more latency than redoing the handshake. Therefore clients typically benefit by
// using a cache and do not incur a penalty if the cached information is outdated.
//
// A SessionCache is tied to a specific client private key. If the SessionCache is used in a
// connection with a different private key, authentication will fail and the vehicle will send
// correct session data as normal.
//
// The same SessionCache may safely be used with different VINs.
//
// If a SessionCache is exported using its [SessionCache.Export] or [SessionCache.ExportToFile]
// methods, access controls should be used to prevent third parties from reading or tampering with
// the data.
package cache
