package util

import (
	"net"
	"strings"

	"golang.org/x/net/idna"
)

// IsUrl reports whether the whole text is a URL.
// An explicit http:// or https:// prefix matches immediately, including internationalized hosts.
// Any other value must be a bare domain or IPv4 address, with an optional port and path.
func IsUrl(raw string) bool {
	value := strings.TrimSpace(raw)
	if value == "" {
		return false
	}
	if hasExplicitHTTPURLScheme(value) {
		return true
	}
	// Bare hosts always contain a dot. Skip domain parsing for large clipboard texts that cannot match.
	if !strings.Contains(value, ".") {
		return false
	}
	return isBareURL(value)
}

// hasExplicitHTTPURLScheme reports an http:// or https:// prefix. Scheme case is ignored.
func hasExplicitHTTPURLScheme(value string) bool {
	return hasASCIIPrefixFold(value, "http://") || hasASCIIPrefixFold(value, "https://")
}

func hasASCIIPrefixFold(value, prefix string) bool {
	if len(value) < len(prefix) {
		return false
	}
	for i := 0; i < len(prefix); i++ {
		c := value[i]
		if c >= 'A' && c <= 'Z' {
			c += 'a' - 'A'
		}
		if c != prefix[i] {
			return false
		}
	}
	return true
}

// isBareURL accepts a domain or IPv4 host without a scheme.
// The host is validated as an internationalized domain name, so labels such as 一元机场 match.
func isBareURL(value string) bool {
	if strings.ContainsAny(value, " \t\r\n") {
		return false
	}
	hostport := value
	if slash := strings.IndexByte(value, '/'); slash >= 0 {
		hostport = value[:slash]
	}
	// A DNS host is at most 253 bytes, plus a colon and a 5-digit port.
	// Punycode is longer than the typed Unicode, so anything already past this limit cannot match.
	if hostport == "" || len(hostport) > 253+6 {
		return false
	}
	host, port, ok := splitBareURLHostPort(hostport)
	if !ok {
		return false
	}
	if port != "" && !validURLPort(port) {
		return false
	}
	if ip := net.ParseIP(host); ip != nil && ip.To4() != nil {
		return true
	}
	return isInternationalDomain(host)
}

func splitBareURLHostPort(hostport string) (string, string, bool) {
	colon := strings.LastIndexByte(hostport, ':')
	if colon < 0 {
		return hostport, "", true
	}
	if strings.IndexByte(hostport, ':') != colon {
		return "", "", false
	}
	host, port := hostport[:colon], hostport[colon+1:]
	if host == "" || port == "" {
		return "", "", false
	}
	return host, port, true
}

func validURLPort(port string) bool {
	if len(port) == 0 || len(port) > 5 {
		return false
	}
	number := 0
	for _, c := range port {
		if c < '0' || c > '9' {
			return false
		}
		number = number*10 + int(c-'0')
	}
	return number >= 1 && number <= 65535
}

// isInternationalDomain accepts a dotted host whose labels survive IDNA conversion.
// The final label needs two ASCII letters, which allows .asia and .museum and rejects a single-letter suffix.
func isInternationalDomain(host string) bool {
	ascii, err := idna.Lookup.ToASCII(host)
	if err != nil || ascii == "" || len(ascii) > 253 || strings.Contains(ascii, "..") {
		return false
	}
	labels := strings.Split(ascii, ".")
	if len(labels) < 2 {
		return false
	}
	for _, label := range labels {
		if !validDomainLabel(label) {
			return false
		}
	}
	letters := 0
	for _, c := range labels[len(labels)-1] {
		if c >= 'a' && c <= 'z' {
			letters++
		}
	}
	return letters >= 2
}

func validDomainLabel(label string) bool {
	if len(label) < 1 || len(label) > 63 || label[0] == '-' || label[len(label)-1] == '-' {
		return false
	}
	for _, c := range label {
		if (c < 'a' || c > 'z') && (c < '0' || c > '9') && c != '-' {
			return false
		}
	}
	return true
}

// NormalizeUrl prepares a URL-like value for shell opening. Bare domains and IP
// addresses keep the URL plugin's existing HTTPS default, while explicit HTTP
// and HTTPS links are left unchanged.
func NormalizeUrl(raw string) string {
	normalized := strings.TrimSpace(raw)
	if hasExplicitHTTPURLScheme(normalized) {
		return normalized
	}
	return "https://" + normalized
}
