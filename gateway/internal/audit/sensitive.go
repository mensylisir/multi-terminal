package audit

import (
	"regexp"
)

var (
	// Patterns for sensitive data
	passwordRegex    = regexp.MustCompile(`(?i)(password|passwd|pwd)\s*[:=]\s*['"]?([^'"\s]+)`)
	secretRegex      = regexp.MustCompile(`(?i)(secret|api_key|apikey|token)\s*[:=]\s*['"]?([^'"\s]+)`)
	credentialRegex  = regexp.MustCompile(`mysql\s+-u\s+(\S+)\s+-p\s+(\S+)`)
)

const mask = "***MASKED***"

type SensitiveFilter struct{}

func NewSensitiveFilter() *SensitiveFilter {
	return &SensitiveFilter{}
}

// Filter masks sensitive information in command strings
func (f *SensitiveFilter) Filter(input string) string {
	result := input

	// Filter password patterns
	result = passwordRegex.ReplaceAllString(result, "${1}: " + mask)

	// Filter secret patterns
	result = secretRegex.ReplaceAllString(result, "${1}: " + mask)

	// Filter mysql credentials
	result = credentialRegex.ReplaceAllString(result, "mysql -u ${1} -p " + mask)

	return result
}

// IsSensitive returns true if the input contains sensitive patterns
func (f *SensitiveFilter) IsSensitive(input string) bool {
	return passwordRegex.MatchString(input) ||
		secretRegex.MatchString(input) ||
		credentialRegex.MatchString(input)
}