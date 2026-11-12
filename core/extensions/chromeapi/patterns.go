//go:build extensions

package chromeapi

import (
	"net/url"
	"strings"
)

// matchURL says whether an address matches one of Chrome's match patterns:
// "<all_urls>", or scheme://host/path, where the scheme may be * (http or https), the host
// * or *.domain, with an optional :port or :*, and the path a glob over path and query.
// https://developer.chrome.com/docs/extensions/develop/concepts/match-patterns
func matchURL(pattern, address string) bool {
	u, err := url.Parse(address)
	if err != nil || u.Scheme == "" {
		return false
	}
	if pattern == "<all_urls>" {
		return allURLsSchemes[u.Scheme]
	}
	scheme, rest, ok := strings.Cut(pattern, "://")
	if !ok {
		return false
	}
	switch {
	case scheme == "*":
		if u.Scheme != "http" && u.Scheme != "https" {
			return false
		}
	case scheme != u.Scheme:
		return false
	}
	host, path, ok := strings.Cut(rest, "/")
	if !ok {
		return false
	}
	if scheme != "file" && !matchHost(host, u) {
		return false
	}
	p := u.EscapedPath()
	if p == "" {
		p = "/"
	}
	if u.RawQuery != "" {
		p += "?" + u.RawQuery
	}
	return glob("/"+path, p, false)
}

var allURLsSchemes = map[string]bool{"http": true, "https": true, "ws": true, "wss": true, "file": true, "ftp": true}

func matchHost(pattern string, u *url.URL) bool {
	host, port, hasPort := strings.Cut(pattern, ":")
	if hasPort && port != "*" && port != u.Port() {
		return false
	}
	name := strings.ToLower(u.Hostname())
	host = strings.ToLower(host)
	switch {
	case host == "*":
		return name != ""
	case strings.HasPrefix(host, "*."):
		domain := host[2:]
		return name == domain || strings.HasSuffix(name, "."+domain)
	}
	return name == host
}

// matchTitle says whether a title matches a pattern of * and ?, as Chrome's
// base::MatchPattern does: case matters, and \ makes the next character plain.
func matchTitle(pattern, title string) bool {
	return glob(pattern, title, true)
}

// glob matches s against a pattern where * is any run of characters, and, when
// question is set, ? any one and \ escapes.
func glob(pattern, s string, question bool) bool {
	p, t := []rune(pattern), []rune(s)
	star, mark := -1, 0
	i, j := 0, 0
	for j < len(t) {
		switch {
		case i < len(p) && p[i] == '*':
			star, mark = i, j
			i++
			continue
		case i < len(p) && question && p[i] == '\\' && i+1 < len(p) && p[i+1] == t[j]:
			i += 2
			j++
			continue
		case i < len(p) && (p[i] == t[j] && !(question && p[i] == '\\') || question && p[i] == '?'):
			i++
			j++
			continue
		case star >= 0:
			i = star + 1
			mark++
			j = mark
			continue
		}
		return false
	}
	for i < len(p) && p[i] == '*' {
		i++
	}
	return i == len(p)
}
