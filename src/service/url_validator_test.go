package service

import (
	"net/url"
	"strings"
	"testing"

	"urlshortener/src/domain"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func mustParse(t *testing.T, raw string) *url.URL {
	t.Helper()
	u, err := url.Parse(raw)
	require.NoError(t, err)
	return u
}

// urlOfLength builds an https URL that is exactly n characters long, padding the path with fill.
func urlOfLength(n int, fill string) string {
	prefix := "https://example.com/"
	return prefix + strings.Repeat(fill, n-len(prefix))
}

func assertInvalid(t *testing.T, err error, wantReason string) {
	t.Helper()
	var invalidURLErr *domain.InvalidURLError
	require.ErrorAs(t, err, &invalidURLErr)
	assert.Equal(t, wantReason, invalidURLErr.Reason)
	assert.Equal(t, "invalid url: "+wantReason, err.Error())
}

func TestValidateLongURL_Valid(t *testing.T) {
	base := mustParse(t, "https://sho.rt/")
	tests := []struct {
		name string
		raw  string
		want string
	}{
		{"https", "https://example.com", "https://example.com"},
		{"http", "http://example.com", "http://example.com"},
		{"trailing slash", "https://example.com/", "https://example.com/"},
		{"uppercase scheme and host", "HTTPS://Example.COM/Path", "HTTPS://Example.COM/Path"},
		{"path, query and fragment", "https://example.com/a/b?q=1&r=two#section", "https://example.com/a/b?q=1&r=two#section"},
		{"subdomains", "https://a.b.example.co.uk/x", "https://a.b.example.co.uk/x"},
		{"hyphenated host", "https://my-site.example.com", "https://my-site.example.com"},
		{"digits in host", "https://web2.example.com", "https://web2.example.com"},
		{"explicit port", "https://example.com:8443/x", "https://example.com:8443/x"},
		{"max port", "https://example.com:65535", "https://example.com:65535"},
		{"trailing dot fqdn", "https://example.com./x", "https://example.com./x"},
		{"punycode host", "https://xn--mnchen-3ya.de", "https://xn--mnchen-3ya.de"},
		{"unicode host", "https://münchen.de", "https://münchen.de"},
		{"cjk host", "https://例え.jp/パス", "https://例え.jp/パス"},
		{"percent encoded path", "https://example.com/a%20b", "https://example.com/a%20b"},
		{"encoded query", "https://example.com/?q=hello%20world", "https://example.com/?q=hello%20world"},
		{"public ipv4", "https://8.8.8.8/dns", "https://8.8.8.8/dns"},
		{"public ipv6", "https://[2001:4860:4860::8888]/", "https://[2001:4860:4860::8888]/"},
		{"63 char label", "https://" + strings.Repeat("a", 63) + ".com", "https://" + strings.Repeat("a", 63) + ".com"},
		{"leading and trailing whitespace trimmed", " \t https://example.com/x \n", "https://example.com/x"},
		{"other site with similar name", "https://sho.rt.example.com/x", "https://sho.rt.example.com/x"},
		{"other site sharing suffix", "https://notsho.rt/x", "https://notsho.rt/x"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := validateLongURL(tt.raw, base)

			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestValidateLongURL_Length(t *testing.T) {
	t.Run("exactly max length", func(t *testing.T) {
		raw := urlOfLength(MaxURLLength, "a")
		require.Len(t, raw, MaxURLLength)

		got, err := validateLongURL(raw, nil)

		require.NoError(t, err)
		assert.Equal(t, raw, got)
	})

	t.Run("one over max length", func(t *testing.T) {
		_, err := validateLongURL(urlOfLength(MaxURLLength+1, "a"), nil)

		assertInvalid(t, err, reasonTooLong)
	})

	t.Run("max length counted in characters, not bytes", func(t *testing.T) {
		raw := "https://example.com/" + strings.Repeat("é", MaxURLLength-len("https://example.com/"))
		require.Greater(t, len(raw), MaxURLLength)

		_, err := validateLongURL(raw, nil)

		require.NoError(t, err)
	})

	t.Run("whitespace padding does not count", func(t *testing.T) {
		raw := "   " + urlOfLength(MaxURLLength, "a") + "   "

		_, err := validateLongURL(raw, nil)

		require.NoError(t, err)
	})
}

func TestValidateLongURL_Invalid(t *testing.T) {
	base := mustParse(t, "https://sho.rt/")
	tests := []struct {
		name   string
		raw    string
		reason string
	}{
		{"empty", "", reasonRequired},
		{"spaces only", "   ", reasonRequired},
		{"tabs and newlines only", "\t\n\r", reasonRequired},

		{"space in host", "https://exa mple.com", reasonWhitespace},
		{"space in path", "https://example.com/a b", reasonWhitespace},
		{"tab in path", "https://example.com/a\tb", reasonWhitespace},
		{"newline in path", "https://example.com/a\nb", reasonWhitespace},
		{"null byte", "https://example.com/\x00", reasonWhitespace},
		{"non-breaking space", "https://example.com/a b", reasonWhitespace},
		{"delete control char", "https://example.com/\x7f", reasonWhitespace},

		{"invalid percent escape", "https://example.com/%zz", reasonMalformed},
		{"unclosed ipv6 bracket", "http://[::1", reasonMalformed},
		{"non-numeric port", "https://example.com:port", reasonMalformed},
		{"missing scheme with colon in first segment", "://example.com", reasonMalformed},

		{"no scheme", "example.com", reasonScheme},
		{"www without scheme", "www.example.com", reasonScheme},
		{"scheme relative", "//example.com/x", reasonScheme},
		{"relative path", "/relative/path", reasonScheme},
		{"ftp", "ftp://example.com/file", reasonScheme},
		{"javascript", "javascript:alert(1)", reasonScheme},
		{"data uri", "data:text/html,<h1>hi</h1>", reasonScheme},
		{"mailto", "mailto:someone@example.com", reasonScheme},
		{"file", "file:///etc/passwd", reasonScheme},
		{"misspelled scheme", "htps://example.com", reasonScheme},
		{"scheme lookalike", "httpx://example.com", reasonScheme},

		{"scheme only", "https://", reasonHost},
		{"empty authority with path", "http:///path", reasonHost},
		{"port without host", "https://:8080", reasonHost},
		{"opaque http url", "http:example.com", reasonHost},

		{"username and password", "https://user:pass@example.com", reasonCredentials},
		{"username only", "https://user@example.com", reasonCredentials},
		{"host spoofing with userinfo", "https://google.com@evil.example", reasonCredentials},

		{"port zero", "https://example.com:0", reasonPort},
		{"port too large", "https://example.com:65536", reasonPort},
		{"port way too large", "https://example.com:99999", reasonPort},

		{"leading hyphen label", "https://-example.com", reasonInvalidHost},
		{"trailing hyphen label", "https://example-.com", reasonInvalidHost},
		{"underscore in host", "https://exa_mple.com", reasonInvalidHost},
		{"empty label", "https://example..com", reasonInvalidHost},
		{"leading dot", "https://.example.com", reasonInvalidHost},
		{"64 char label", "https://" + strings.Repeat("a", 64) + ".com", reasonInvalidHost},
		{"hostname over 253 chars", "https://" + strings.Repeat("a.", 127) + "com", reasonInvalidHost},
		{"ipv4 octet out of range", "https://999.1.1.1", reasonInvalidHost},
		{"too few ipv4 octets", "https://1.2.3", reasonInvalidHost},
		{"numeric tld", "https://example.123", reasonInvalidHost},
		{"invalid ipv6", "https://[12345::1]/", reasonMalformed},

		{"localhost", "http://localhost:8080/x", reasonNoTLD},
		{"single label host", "https://intranet/", reasonNoTLD},
		{"bare tld", "https://com", reasonNoTLD},

		{"ipv4 loopback", "http://127.0.0.1/x", reasonPrivateIP},
		{"ipv4 loopback range", "http://127.1.2.3/x", reasonPrivateIP},
		{"10/8 private", "http://10.0.0.1/", reasonPrivateIP},
		{"172.16/12 private", "http://172.16.0.1/", reasonPrivateIP},
		{"192.168/16 private", "http://192.168.1.1/", reasonPrivateIP},
		{"link local", "http://169.254.169.254/latest/meta-data", reasonPrivateIP},
		{"unspecified", "http://0.0.0.0/", reasonPrivateIP},
		{"broadcast", "http://255.255.255.255/", reasonPrivateIP},
		{"multicast", "http://224.0.0.1/", reasonPrivateIP},
		{"ipv6 loopback", "http://[::1]/", reasonPrivateIP},
		{"ipv6 unspecified", "http://[::]/", reasonPrivateIP},
		{"ipv6 link local", "http://[fe80::1]/", reasonPrivateIP},
		{"ipv6 unique local", "http://[fc00::1]/", reasonPrivateIP},
		{"ipv4-mapped ipv6 loopback", "http://[::ffff:127.0.0.1]/", reasonPrivateIP},
		{"ipv6 link local with zone", "http://[fe80::1%25en0]/", reasonPrivateIP},

		{"self link short code", "https://sho.rt/abc123", reasonSelfLink},
		{"self link root", "https://sho.rt", reasonSelfLink},
		{"self link other scheme", "http://sho.rt/abc123", reasonSelfLink},
		{"self link uppercase host", "https://SHO.RT/abc123", reasonSelfLink},
		{"self link other port", "https://sho.rt:8443/abc123", reasonSelfLink},
		{"self link trailing dot", "https://sho.rt./abc123", reasonSelfLink},
		{"self link with whitespace", "  https://sho.rt/abc123  ", reasonSelfLink},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := validateLongURL(tt.raw, base)

			assertInvalid(t, err, tt.reason)
			assert.Empty(t, got)
		})
	}
}

func TestValidateLongURL_SelfLinkWithBasePath(t *testing.T) {
	base := mustParse(t, "https://example.com/s/")
	tests := []struct {
		raw      string
		selfLink bool
	}{
		{"https://example.com/s/abc123", true},
		{"https://example.com/s/", true},
		{"https://example.com/s", true},
		{"https://example.com/s/a/b", true},
		{"https://example.com/", false},
		{"https://example.com", false},
		{"https://example.com/about", false},
		{"https://example.com/short", false},
		{"https://example.com/x/s/abc", false},
		{"https://other.com/s/abc123", false},
	}

	for _, tt := range tests {
		t.Run(tt.raw, func(t *testing.T) {
			_, err := validateLongURL(tt.raw, base)

			if tt.selfLink {
				assertInvalid(t, err, reasonSelfLink)
				return
			}
			assert.NoError(t, err)
		})
	}
}

func TestValidateLongURL_SelfLinkCheckedBeforeHostRules(t *testing.T) {
	// with a localhost base URL, links back to it get the clearer self-link reason
	base := mustParse(t, "http://localhost:8080/")

	_, err := validateLongURL("http://localhost:8080/abc123", base)

	assertInvalid(t, err, reasonSelfLink)
}

func TestValidateLongURL_NoSelfLinkCheckWithoutBase(t *testing.T) {
	for name, base := range map[string]*url.URL{"nil base": nil, "base without host": {}} {
		t.Run(name, func(t *testing.T) {
			got, err := validateLongURL("https://sho.rt/abc123", base)

			require.NoError(t, err)
			assert.Equal(t, "https://sho.rt/abc123", got)
		})
	}
}

func TestCheckLongURL_RejectsWithSameReasonAsShorten(t *testing.T) {
	tests := []struct {
		name   string
		raw    string
		reason string
	}{
		{"empty", "", reasonRequired},
		{"not a url", "not a url", reasonWhitespace},
		{"no scheme", "www.test.com", reasonScheme},
		{"javascript", "javascript:alert(1)", reasonScheme},
		{"data", "data:text/html,<script>alert(1)</script>", reasonScheme},
		{"ftp", "ftp://example.com", reasonScheme},
		{"mailto", "mailto:a@b.c", reasonScheme},
		{"no host", "http://", reasonHost},
		{"too long", urlOfLength(MaxURLLength+1, "a"), reasonTooLong},
		{"credentials", "https://user:pass@example.com", reasonCredentials},
		{"bad port", "https://example.com:0", reasonPort},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assertInvalid(t, CheckLongURL(tt.raw), tt.reason)
		})
	}
}

func TestCheckLongURL_LeavesContextDependentRulesToShorten(t *testing.T) {
	// self links need the base URL and host rules run after the self-link check,
	// so both are only enforced by Shorten
	for _, raw := range []string{
		"https://example.com/a?b=c",
		"HTTP://EXAMPLE.COM",
		"  https://example.com  ",
		"http://localhost:8080/abc123",
		"http://127.0.0.1",
	} {
		t.Run(raw, func(t *testing.T) {
			assert.NoError(t, CheckLongURL(raw))
		})
	}
}

func TestIsValidLabel(t *testing.T) {
	tests := []struct {
		label string
		want  bool
	}{
		{"example", true},
		{"EXAMPLE", true},
		{"web2", true},
		{"my-site", true},
		{"xn--mnchen-3ya", true},
		{"a", true},
		{strings.Repeat("a", 63), true},
		{"", false},
		{strings.Repeat("a", 64), false},
		{"-example", false},
		{"example-", false},
		{"-", false},
		{"exa_mple", false},
		{"exa mple", false},
		{"münchen", false},
		{"exa$mple", false},
	}

	for _, tt := range tests {
		t.Run(tt.label, func(t *testing.T) {
			assert.Equal(t, tt.want, isValidLabel(tt.label))
		})
	}
}
