package protocol

import (
	universal "github.com/teslamotors/vehicle-command/pkg/protocol/protobuf/universalmessage"
)

// Domain identifies the vehicle subsystem to route a command to. Each Domain manages its own key
// pair.
type Domain = universal.Domain

const (
	DomainNone = universal.Domain_DOMAIN_BROADCAST
	// DomainVCSEC handles (un)lock, remote start drive, keychain management commands.
	DomainVCSEC = universal.Domain_DOMAIN_VEHICLE_SECURITY
	// DomainInfotainment handles commands that terminate on the vehicle's infotainment system.
	DomainInfotainment = universal.Domain_DOMAIN_INFOTAINMENT
)
