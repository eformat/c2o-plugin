package handlers

import "regexp"

var (
	namespaceRe  = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{0,61}[a-z0-9]$`)
	prefixRe     = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{0,29}$`)
	secretNameRe = regexp.MustCompile(`^[a-z0-9][a-z0-9.-]{0,251}[a-z0-9]$`)
	imageRe      = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9._\-/:@]{0,254}$`)

	validAgentTypes = map[string]bool{
		"claude":   true,
		"codex":    true,
		"opencode": true,
	}
)

func isValidNamespace(s string) bool {
	return len(s) >= 2 && namespaceRe.MatchString(s)
}

func isValidPrefix(s string) bool {
	return len(s) >= 1 && prefixRe.MatchString(s)
}

func isValidSecretName(s string) bool {
	return len(s) >= 2 && secretNameRe.MatchString(s)
}

func isValidAgentType(s string) bool {
	return validAgentTypes[s]
}

func isValidImage(s string) bool {
	return len(s) >= 1 && imageRe.MatchString(s)
}
