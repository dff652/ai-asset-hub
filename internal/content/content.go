package content

import "regexp"

// MaxScannedFileSize is the Phase 0/1 read limit for content inspection.
const MaxScannedFileSize = 10 << 20

var secretPatterns = []*regexp.Regexp{
	regexp.MustCompile(`(?i)\bsk-[a-z0-9_-]{12,}`),
	regexp.MustCompile(`\bAKIA[A-Z0-9]{16}\b`),
	regexp.MustCompile(`(?i)\b(?:ghp_|github_pat_)[a-z0-9_]{12,}`),
	regexp.MustCompile(`(?i)\bbearer\s+[a-z0-9_./+=-]{8,}`),
	regexp.MustCompile(`(?i)(?:api[_-]?key|token|authorization|password|secret)\s*["']?\s*[:=]\s*["']?[a-z0-9_./+=-]{8,}`),
	regexp.MustCompile(`-----BEGIN (?:RSA |EC |OPENSSH )?PRIVATE KEY-----`),
}

// ContainsSecret reports whether content matches a known secret pattern.
// Callers must not include matching substrings in logs or reports.
func ContainsSecret(content []byte) bool {
	for _, pattern := range secretPatterns {
		if pattern.Match(content) {
			return true
		}
	}
	return false
}

// IsBinary reports whether the leading bytes look like binary content.
func IsBinary(content []byte) bool {
	limit := len(content)
	if limit > 8000 {
		limit = 8000
	}
	for _, value := range content[:limit] {
		if value == 0 {
			return true
		}
	}
	return false
}
