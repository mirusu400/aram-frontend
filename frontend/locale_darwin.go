//go:build darwin

package frontend

import (
	"os"
	"os/exec"
	"strings"
)

func osLocale() string {
	if output, err := exec.Command(
		"defaults",
		"read",
		"-g",
		"AppleLanguages",
	).Output(); err == nil {
		for _, line := range strings.Split(string(output), "\n") {
			line = strings.Trim(strings.TrimSpace(line), "\",")
			if line != "" && line != "(" && line != ")" {
				return line
			}
		}
	}
	for _, name := range []string{"LC_ALL", "LC_MESSAGES", "LANG", "LANGUAGE"} {
		if value := os.Getenv(name); value != "" {
			return value
		}
	}
	return ""
}
