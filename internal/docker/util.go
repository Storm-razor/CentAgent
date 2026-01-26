package docker

import "strings"

func truncateID(id string) string {
	id = strings.TrimSpace(id)
	if len(id) <= 12 {
		return id
	}
	return id[:12]
}

func truncateTail(s string, maxLen int) string {
	if maxLen <= 0 {
		return ""
	}
	if len(s) <= maxLen {
		return s
	}
	return "...(truncated)...\n" + s[len(s)-maxLen:]
}
