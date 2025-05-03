package sign

import (
	"github.com/golang-jwt/jwt/v5"
	"github.com/teslamotors/vehicle-command/internal/authentication"
)

// MessageForVehicle returns a JWT with the provided claims. Only the vehicle with the given VIN
// will accept the JWT. To create a JWT that is valid for all vehicles in a fleet, use
// [MessageForFleet].
//
// The function overwrites the audience ("aud") and issuer ("iss") JWT claims.
func MessageForVehicle(privateKey authentication.ECDHPrivateKey, vin, app string, message jwt.MapClaims) (string, error) {
	return authentication.SignMessage(privateKey, message, "com.tesla.vehicle."+vin+"."+app)
}

// MessageForFleet returns a JWT with the provided claims. All vehicles that trust privateKey
// will accept the JWT. To create a JWT that is valid for a single vehicle, use
// [MessageForVehicle].
//
// The function overwrites the audience ("aud") and issuer ("iss") JWT claims.
func MessageForFleet(privateKey authentication.ECDHPrivateKey, app string, message jwt.MapClaims) (string, error) {
	// Issuers are identified by their public key
	return authentication.SignMessage(privateKey, message, "com.tesla.fleet."+app)
}
