package audit

import (
	"math"
	"regexp"
	"strings"
)

// Rule describes one kind of secret. If Group is non-zero, that capture
// group is the secret itself and is what MinEntropy is checked against.
type Rule struct {
	ID          string
	Description string
	Regex       *regexp.Regexp
	Group       int
	MinEntropy  float64
	// Filter, if set, must return true for the candidate to be reported.
	Filter func(secret string) bool
}

// looksGenerated rejects code such as crypto.HashPassword and plain words:
// generated credentials almost always mix letters and digits.
func looksGenerated(s string) bool {
	return strings.ContainsAny(s, "0123456789") && strings.ContainsFunc(s, func(r rune) bool {
		return (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z')
	})
}

// DefaultRules target credential formats with distinctive prefixes, plus a
// generic key=value rule gated on entropy to keep false positives low.
var DefaultRules = []Rule{
	{ID: "aws-access-key-id", Description: "AWS access key ID",
		Regex: regexp.MustCompile(`\b((?:AKIA|ASIA)[0-9A-Z]{16})\b`)},
	{ID: "aws-secret-access-key", Description: "AWS secret access key",
		Regex: regexp.MustCompile(`(?i)aws.{0,20}secret.{0,20}?[=:]\s*['"]?([0-9a-zA-Z/+]{40})\b`), Group: 1, MinEntropy: 4},
	{ID: "github-token", Description: "GitHub token",
		Regex: regexp.MustCompile(`\b(gh[pousr]_[0-9a-zA-Z]{36,255}|github_pat_[0-9a-zA-Z_]{82})\b`)},
	{ID: "gitlab-token", Description: "GitLab personal access token",
		Regex: regexp.MustCompile(`\bglpat-[0-9a-zA-Z_-]{20}\b`)},
	{ID: "slack-token", Description: "Slack token",
		Regex: regexp.MustCompile(`\bxox[abposr]-[0-9A-Za-z-]{10,}`)},
	{ID: "slack-webhook", Description: "Slack incoming webhook",
		Regex: regexp.MustCompile(`https://hooks\.slack\.com/services/T[0-9A-Z]+/B[0-9A-Z]+/[0-9A-Za-z]+`)},
	{ID: "stripe-live-key", Description: "Stripe live secret or restricted key",
		Regex: regexp.MustCompile(`\b[rs]k_live_[0-9a-zA-Z]{24,}\b`)},
	{ID: "google-api-key", Description: "Google API key",
		Regex: regexp.MustCompile(`\bAIza[0-9A-Za-z_-]{35}\b`)},
	{ID: "openai-api-key", Description: "OpenAI API key",
		Regex: regexp.MustCompile(`\bsk-(?:proj|svcacct|admin)-[A-Za-z0-9_-]{40,}`)},
	{ID: "anthropic-api-key", Description: "Anthropic API key",
		Regex: regexp.MustCompile(`\bsk-ant-(?:api|admin)\d{2}-[A-Za-z0-9_-]{80,}`)},
	{ID: "sendgrid-api-key", Description: "SendGrid API key",
		Regex: regexp.MustCompile(`\bSG\.[A-Za-z0-9_-]{22}\.[A-Za-z0-9_-]{43}\b`)},
	{ID: "twilio-api-key", Description: "Twilio API key",
		Regex: regexp.MustCompile(`\bSK[0-9a-f]{32}\b`)},
	{ID: "npm-token", Description: "npm access token",
		Regex: regexp.MustCompile(`\bnpm_[A-Za-z0-9]{36}\b`)},
	{ID: "private-key", Description: "Private key block",
		Regex: regexp.MustCompile(`-----BEGIN (?:RSA |EC |DSA |OPENSSH |PGP |ENCRYPTED )?PRIVATE KEY(?: BLOCK)?-----`)},
	{ID: "jwt", Description: "JSON Web Token",
		Regex: regexp.MustCompile(`\beyJ[A-Za-z0-9_-]{10,}\.eyJ[A-Za-z0-9_-]{10,}\.[A-Za-z0-9_-]{10,}`)},
	{ID: "connection-string", Description: "Connection string with embedded password",
		Regex: regexp.MustCompile(`\b(?:postgres(?:ql)?|mysql|mongodb(?:\+srv)?|redis|rediss|amqps?)://[^:\s/@]+:([^@\s/]{6,})@`), Group: 1, MinEntropy: 3},
	{ID: "generic-secret", Description: "Generic secret assignment",
		Regex: regexp.MustCompile(`(?i)\b[A-Z0-9_]*(?:api[_-]?key|secret|token|passw(?:or)?d|access[_-]?key|private[_-]?key)[A-Z0-9_]*\s*[:=]\s*['"]?([A-Za-z0-9+/=_.~!@#$%^&*?-]{16,})`), Group: 1, MinEntropy: 3.5, Filter: looksGenerated},
}

// placeholderHints mark values that are documentation, not real secrets.
var placeholderHints = []string{"example", "changeme", "placeholder", "your_", "your-", "xxxxxx", "redacted", "dummy", "${", "<"}

func (r Rule) match(line string) (string, bool) {
	m := r.Regex.FindStringSubmatch(line)
	if m == nil {
		return "", false
	}
	secret := m[0]
	if r.Group > 0 && r.Group < len(m) {
		secret = m[r.Group]
	}
	if r.Filter != nil && !r.Filter(secret) {
		return "", false
	}
	if r.MinEntropy > 0 {
		if entropy(secret) < r.MinEntropy {
			return "", false
		}
		lower := strings.ToLower(secret)
		for _, h := range placeholderHints {
			if strings.Contains(lower, h) {
				return "", false
			}
		}
	}
	return secret, true
}

// entropy returns the Shannon entropy of s in bits per character.
func entropy(s string) float64 {
	if s == "" {
		return 0
	}
	counts := make(map[rune]int)
	n := 0
	for _, r := range s {
		counts[r]++
		n++
	}
	var h float64
	for _, c := range counts {
		p := float64(c) / float64(n)
		h -= p * math.Log2(p)
	}
	return h
}

// Redact keeps just enough of a secret to identify it.
func Redact(secret string) string {
	r := []rune(secret)
	if len(r) <= 8 {
		return strings.Repeat("*", len(r))
	}
	return string(r[:4]) + strings.Repeat("*", 8)
}
