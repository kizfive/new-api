package common

import "strings"

// ParseCPARequestID extracts the request id from CPA's X-CPA-TRACE-ID value.
// CPA formats it as <selection time>-<auth index>-<request id>; the auth
// index may itself contain hyphens, while the request id is the trailing
// bare token. An absent or malformed value returns "".
func ParseCPARequestID(trace string) string {
	trace = strings.TrimSpace(trace)
	if len(trace) >= 15 && trace[14] == '-' && isDigits(trace[:14]) {
		if len(trace) >= 36 && isUUID(trace[len(trace)-36:]) {
			return trace[len(trace)-36:]
		}
	}
	index := strings.LastIndexByte(trace, '-')
	if index < 0 || index == len(trace)-1 {
		return ""
	}
	id := trace[index+1:]
	if len(id) > 64 {
		return ""
	}
	for _, character := range id {
		if !((character >= '0' && character <= '9') || (character >= 'a' && character <= 'z') || (character >= 'A' && character <= 'Z')) {
			return ""
		}
	}
	return id
}

func isDigits(value string) bool {
	for _, character := range value {
		if character < '0' || character > '9' {
			return false
		}
	}
	return true
}

// isUUID accepts the canonical 8-4-4-4-12 hexadecimal shape used by CPA's
// UUIDv7 request ids.
func isUUID(value string) bool {
	if len(value) != 36 {
		return false
	}
	for index, character := range value {
		switch index {
		case 8, 13, 18, 23:
			if character != '-' {
				return false
			}
		default:
			if !((character >= '0' && character <= '9') ||
				(character >= 'a' && character <= 'f') ||
				(character >= 'A' && character <= 'F')) {
				return false
			}
		}
	}
	return true
}
