package service

import (
	"net/netip"
	"net/url"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"
	"urlshortener/src/domain"

	"golang.org/x/net/idna"
)

// MaxURLLength matches the LONG_URL varchar(1000) column.
const MaxURLLength = 1000

const (
	reasonRequired       = "url is required"
	reasonTooLong        = "url must be at most 1000 characters"
	reasonWhitespace     = "url must not contain spaces or control characters"
	reasonMalformed      = "url is malformed"
	reasonScheme         = "url must start with http:// or https://"
	reasonHost           = "url must include a host"
	reasonCredentials    = "url must not contain a username or password" //nolint:gosec // G101: an error message, not a credential
	reasonPort           = "url port must be between 1 and 65535"
	reasonInvalidHost    = "url host is not a valid domain name or IP address"
	reasonNoTLD          = "url host must be a public domain like example.com"
	reasonPrivateIP      = "url must not point to a private, loopback or reserved IP address"
	reasonSelfLink       = "url must not point to this url shortener"
	maxHostnameLength    = 253
	maxHostnameLabelSize = 63
)

func invalid(reason string) error {
	return &domain.InvalidURLError{Reason: reason}
}

// validateLongURL checks that raw is a shareable absolute http(s) URL and returns it
// with surrounding whitespace removed. base is the shortener's own public URL, used
// to reject links back to the shortener (which could create redirect loops); it may be nil.
func validateLongURL(raw string, base *url.URL) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", invalid(reasonRequired)
	}
	if utf8.RuneCountInString(raw) > MaxURLLength {
		return "", invalid(reasonTooLong)
	}
	if strings.ContainsFunc(raw, func(r rune) bool { return unicode.IsSpace(r) || unicode.IsControl(r) }) {
		return "", invalid(reasonWhitespace)
	}

	u, err := url.Parse(raw)
	if err != nil {
		return "", invalid(reasonMalformed)
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return "", invalid(reasonScheme)
	}
	if u.Opaque != "" || u.Host == "" || u.Hostname() == "" {
		return "", invalid(reasonHost)
	}
	if u.User != nil {
		return "", invalid(reasonCredentials)
	}
	if port := u.Port(); port != "" {
		if n, err := strconv.Atoi(port); err != nil || n < 1 || n > 65535 {
			return "", invalid(reasonPort)
		}
	}
	if isSelfLink(u, base) {
		return "", invalid(reasonSelfLink)
	}
	if err := validateHost(u.Hostname()); err != nil {
		return "", err
	}

	return raw, nil
}

func validateHost(host string) error {
	if ip, err := netip.ParseAddr(host); err == nil {
		if !isPublicIP(ip) {
			return invalid(reasonPrivateIP)
		}
		return nil
	}
	// a bracketed host that isn't a valid IPv6 address, or an IPv4-looking host like 999.1.1.1
	if strings.Contains(host, ":") || looksNumeric(host) {
		return invalid(reasonInvalidHost)
	}

	ascii, err := idna.Lookup.ToASCII(strings.TrimSuffix(host, "."))
	if err != nil || ascii == "" || len(ascii) > maxHostnameLength {
		return invalid(reasonInvalidHost)
	}
	labels := strings.Split(ascii, ".")
	for _, label := range labels {
		if !isValidLabel(label) {
			return invalid(reasonInvalidHost)
		}
	}
	if len(labels) < 2 {
		return invalid(reasonNoTLD)
	}
	if tld := labels[len(labels)-1]; looksNumeric(tld) {
		return invalid(reasonInvalidHost)
	}
	return nil
}

func isValidLabel(label string) bool {
	if label == "" || len(label) > maxHostnameLabelSize {
		return false
	}
	if label[0] == '-' || label[len(label)-1] == '-' {
		return false
	}
	for _, r := range label {
		if !isLabelChar(r) {
			return false
		}
	}
	return true
}

func isLabelChar(r rune) bool {
	return r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '-'
}

func looksNumeric(s string) bool {
	return strings.Trim(s, "0123456789.") == ""
}

func isPublicIP(ip netip.Addr) bool {
	ip = ip.Unmap()
	return ip.IsGlobalUnicast() && !ip.IsPrivate() && !ip.IsLoopback() && !ip.IsLinkLocalUnicast()
}

// isSelfLink reports whether u points at the shortener itself: same host
// (ignoring case, port and scheme) and a path under the base path.
func isSelfLink(u, base *url.URL) bool {
	if base == nil || base.Hostname() == "" {
		return false
	}
	if !strings.EqualFold(strings.TrimSuffix(u.Hostname(), "."), strings.TrimSuffix(base.Hostname(), ".")) {
		return false
	}
	basePath := strings.TrimSuffix(base.Path, "/")
	path := u.Path
	return basePath == "" || path == basePath || strings.HasPrefix(path, basePath+"/")
}
