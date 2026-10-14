package account

import (
	"net"
	"net/url"
	"strings"

	"github.com/teslamotors/vehicle-command/pkg/protocol"
)

// VirtualKeyInstallURL returns Tesla's hosted enrollment link for a
// registered partner domain: https://tesla.com/_ak/<domain>.
// domain is the hostname registered with Tesla, not a URL and not a
// return path. The result has no query string. See
// teslamotors/vehicle-command#444.
func VirtualKeyInstallURL(domain string) (string, error) {
	host, err := partnerHostname(domain)
	if err != nil {
		return "", err
	}
	return "https://tesla.com/_ak/" + host, nil
}

// RejectVirtualKeyReturnURI reports that this SDK cannot change the Finish
// Setup button on Tesla's hosted _ak page. returnURI is inspected so an
// off-domain value is not treated as a same-partner redirect. A same-domain
// https URL is still refused: Tesla has not published return_uri on that
// page, and appending it here would not move the button. See
// teslamotors/vehicle-command#444.
func RejectVirtualKeyReturnURI(partnerDomain, returnURI string) error {
	_ = VirtualKeyReturnHostAllowed(partnerDomain, returnURI)
	return protocol.ErrVirtualKeyReturnURI
}

// VirtualKeyReturnHostAllowed reports whether returnURI is an https URL
// whose host is partnerDomain or a subdomain of it, with no userinfo.
// Lookalike hosts (example.com.evil.test) are rejected. A true result does
// not mean Tesla's _ak page will honor return_uri.
func VirtualKeyReturnHostAllowed(partnerDomain, returnURI string) bool {
	partner, err := partnerHostname(partnerDomain)
	if err != nil {
		return false
	}
	u, err := url.Parse(strings.TrimSpace(returnURI))
	if err != nil || u.Host == "" || u.User != nil {
		return false
	}
	if !strings.EqualFold(u.Scheme, "https") {
		return false
	}
	host := strings.TrimSuffix(strings.ToLower(u.Hostname()), ".")
	if host == "" || net.ParseIP(host) != nil {
		return false
	}
	if strings.ContainsAny(host, "/\\@:?#") {
		return false
	}
	return host == partner || strings.HasSuffix(host, "."+partner)
}

func partnerHostname(domain string) (string, error) {
	host := strings.TrimSpace(strings.ToLower(domain))
	host = strings.TrimSuffix(host, ".")
	if host == "" || strings.ContainsAny(host, "/\\@:?# \t") || strings.Contains(host, ":") {
		return "", protocol.ErrVirtualKeyReturnURI
	}
	if net.ParseIP(host) != nil {
		return "", protocol.ErrVirtualKeyReturnURI
	}
	labels := strings.Split(host, ".")
	if len(labels) < 2 {
		return "", protocol.ErrVirtualKeyReturnURI
	}
	for _, label := range labels {
		if label == "" || len(label) > 63 {
			return "", protocol.ErrVirtualKeyReturnURI
		}
		if label[0] == '-' || label[len(label)-1] == '-' {
			return "", protocol.ErrVirtualKeyReturnURI
		}
		for _, c := range label {
			if (c < 'a' || c > 'z') && (c < '0' || c > '9') && c != '-' {
				return "", protocol.ErrVirtualKeyReturnURI
			}
		}
	}
	return host, nil
}
