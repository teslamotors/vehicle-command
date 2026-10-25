package account

// Fleet API OAuth scopes. vehicle_location is required for location data and is
// no longer implied by vehicle_device_data.
const (
	ScopeOpenID              = "openid"
	ScopeOfflineAccess       = "offline_access"
	ScopeUserData            = "user_data"
	ScopeVehicleDeviceData   = "vehicle_device_data"
	ScopeVehicleLocation     = "vehicle_location"
	ScopeVehicleCmds         = "vehicle_cmds"
	ScopeVehicleChargingCmds = "vehicle_charging_cmds"
	ScopeEnergyDeviceData    = "energy_device_data"
	ScopeEnergyCmds          = "energy_cmds"
)

// DefaultThirdPartyScopes is the third-party scope set, including
// vehicle_location. Token exchange must send this list; Tesla may omit
// vehicle_location if it is granted at /authorize but not repeated on /token.
var DefaultThirdPartyScopes = []string{
	ScopeOpenID,
	ScopeOfflineAccess,
	ScopeUserData,
	ScopeVehicleDeviceData,
	ScopeVehicleLocation,
	ScopeVehicleCmds,
	ScopeVehicleChargingCmds,
}

// HasScope reports whether want is present in scopes.
func HasScope(scopes []string, want string) bool {
	for _, scope := range scopes {
		if scope == want {
			return true
		}
	}
	return false
}

// HasScope reports whether the account's access token includes scope.
func (a *Account) HasScope(scope string) bool {
	if a == nil {
		return false
	}
	return HasScope(a.Scopes, scope)
}
