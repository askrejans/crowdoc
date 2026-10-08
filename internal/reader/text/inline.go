package text

import (
	"strings"

	"github.com/askrejans/crowdoc/v2/ast"
)

// linkify turns http(s) URLs, "www." addresses and e-mail addresses in s
// into links; trailing punctuation stays outside the link.
func linkify(s string) []ast.Inline {
	var out []ast.Inline
	start := 0
	for i := 0; i < len(s); i++ {
		url, n := urlAt(s, i)
		if n == 0 {
			continue
		}
		out = append(out, emails(s[start:i])...)
		out = append(out, &ast.Link{URL: url, Inlines: []ast.Inline{&ast.Text{Value: s[i : i+n]}}})
		i += n - 1
		start = i + 1
	}
	return append(out, emails(s[start:])...)
}

func isAlnum(c byte) bool {
	return c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9'
}

// urlAt returns the link target and length of a URL starting at s[i].
func urlAt(s string, i int) (string, int) {
	if i > 0 && (isAlnum(s[i-1]) || s[i-1] == '/' || s[i-1] == '.' || s[i-1] == '@') {
		return "", 0
	}
	rest := s[i:]
	lower := strings.ToLower(rest[:min(len(rest), 8)])
	prefix := ""
	switch {
	case strings.HasPrefix(lower, "https://"):
		prefix = "https://"
	case strings.HasPrefix(lower, "http://"):
		prefix = "http://"
	case strings.HasPrefix(lower, "www.") && len(rest) > 4 && isAlnum(rest[4]):
		prefix = "www."
	default:
		return "", 0
	}
	end := len(prefix)
	for end < len(rest) {
		c := rest[end]
		if c == ' ' || c == '\t' || c == '<' || c == '>' || c == '"' || strings.HasPrefix(rest[end:], "\u00a0") {
			break
		}
		end++
	}
	end = trimURL(rest[:end])
	if end <= len(prefix)+1 || !strings.Contains(rest[len(prefix):end], ".") && prefix == "www." {
		return "", 0
	}
	url := rest[:end]
	if prefix == "www." {
		url = "https://" + url
	}
	return url, end
}

// trimURL drops trailing punctuation that belongs to the sentence, keeping
// a closing parenthesis that matches one inside the URL.
func trimURL(u string) int {
	end := len(u)
	for end > 0 {
		c := u[end-1]
		switch {
		case strings.IndexByte(".,;:!?'*", c) >= 0:
			end--
		case c == ')' && strings.Count(u[:end], "(") < strings.Count(u[:end], ")"),
			c == ']' && strings.Count(u[:end], "[") < strings.Count(u[:end], "]"):
			end--
		default:
			return end
		}
	}
	return end
}

// emails links e-mail addresses in plain text.
func emails(s string) []ast.Inline {
	if s == "" {
		return nil
	}
	var out []ast.Inline
	start := 0
	for at := strings.IndexByte(s, '@'); at >= 0; {
		lo := at
		for lo > start && isLocalChar(s[lo-1]) {
			lo--
		}
		hi := at + 1
		for hi < len(s) && (isAlnum(s[hi]) || s[hi] == '.' || s[hi] == '-') {
			hi++
		}
		for hi > at+1 && (s[hi-1] == '.' || s[hi-1] == '-') {
			hi--
		}
		if validEmail(s[lo:at], s[at+1:hi]) {
			if lo > start {
				out = append(out, &ast.Text{Value: s[start:lo]})
			}
			addr := s[lo:hi]
			out = append(out, &ast.Link{URL: "mailto:" + addr, Inlines: []ast.Inline{&ast.Text{Value: addr}}})
			start = hi
		}
		next := strings.IndexByte(s[at+1:], '@')
		if next < 0 {
			break
		}
		at += 1 + next
		if at < start {
			at = start
		}
	}
	if start < len(s) {
		out = append(out, &ast.Text{Value: s[start:]})
	}
	return out
}

func isLocalChar(c byte) bool {
	return isAlnum(c) || strings.IndexByte("._%+-", c) >= 0
}

func validEmail(local, domain string) bool {
	if local == "" || local[0] == '.' || domain == "" || domain[0] == '.' || domain[0] == '-' {
		return false
	}
	dot := strings.LastIndexByte(domain, '.')
	if dot <= 0 {
		return false
	}
	tld := domain[dot+1:]
	if len(tld) < 2 {
		return false
	}
	for i := 0; i < len(tld); i++ {
		if !(tld[i] >= 'a' && tld[i] <= 'z' || tld[i] >= 'A' && tld[i] <= 'Z') {
			return false
		}
	}
	return !strings.Contains(domain, "..")
}
