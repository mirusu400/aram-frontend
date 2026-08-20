//go:build !windows && !darwin

package frontend

import "os"

func osLocale() string {
	for _, name := range []string{"LC_ALL", "LC_MESSAGES", "LANG", "LANGUAGE"} {
		if value := os.Getenv(name); value != "" {
			return value
		}
	}
	return ""
}
