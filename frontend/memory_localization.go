package frontend

import (
	"regexp"
	"strings"
)

// The tool contract carries English display text. Match only its known memory
// message formats, keeping guest addresses, numeric values and region IDs intact.
var memoryMessageFormats = []struct {
	pattern *regexp.Regexp
	message string
}{
	{regexp.MustCompile(`^Wrote (.+) to (0x[0-9a-fA-F]{8})$`), "Wrote %s to %s"},
	{regexp.MustCompile(`^(.+) \((0x[0-9a-fA-F]{8}), ([0-9]+) bytes\)$`), "%s (%s, %s bytes)"},
	{regexp.MustCompile(`^invalid numeric type (.+)$`), "invalid numeric type %s"},
	{regexp.MustCompile(`^("(?:[^"\\]|\\.)*") is not a finite (\w+) value$`), "%s is not a finite %s value"},
	{regexp.MustCompile(`^("(?:[^"\\]|\\.)*") is outside the (\w+) range$`), "%s is outside the %s range"},
	{regexp.MustCompile(`^invalid 32-bit guest address (.+)$`), "invalid 32-bit guest address %s"},
	{regexp.MustCompile(`^invalid comparison (.+)$`), "invalid comparison %s"},
	{regexp.MustCompile(`^region (.+) is not searchable$`), "region %s is not searchable"},
	{regexp.MustCompile(`^unknown memory action (.+)$`), "unknown memory action %s"},
	{regexp.MustCompile(`^comparison ([0-9]+) requires a previous scan$`), "comparison %s requires a previous scan"},
	{regexp.MustCompile(`^comparison ([0-9]+) is invalid for a first scan$`), "comparison %s is invalid for a first scan"},
	{regexp.MustCompile(`^invalid next-scan comparison ([0-9]+)$`), "invalid next-scan comparison %s"},
	{regexp.MustCompile(`^read scan result (0x[0-9a-fA-F]{8})$`), "read scan result %s"},
	{regexp.MustCompile(`^read cheat memory at (0x[0-9a-fA-F]{8})$`), "read cheat memory at %s"},
	{regexp.MustCompile(`^read expected memory at (0x[0-9a-fA-F]{8})$`), "read expected memory at %s"},
	{regexp.MustCompile(`^write cheat memory at (0x[0-9a-fA-F]{8})$`), "write cheat memory at %s"},
}

func (s *Shell) trToolText(kind ToolKind, message string) string {
	if kind == ToolMemory {
		return s.trMemoryText(message)
	}
	return s.tr(message)
}

func (s *Shell) trMemoryText(message string) string {
	if s.language() == LanguageEnglish {
		return message
	}
	if strings.Contains(message, "\n") {
		lines := strings.Split(message, "\n")
		for index, line := range lines {
			lines[index] = s.trMemoryText(line)
		}
		return strings.Join(lines, "\n")
	}
	if translated := s.tr(message); translated != message {
		return translated
	}
	for _, format := range memoryMessageFormats {
		if parts := format.pattern.FindStringSubmatch(message); parts != nil {
			args := make([]any, len(parts)-1)
			for index, part := range parts[1:] {
				args[index] = part
			}
			return s.trf(format.message, args...)
		}
	}
	if head, detail, ok := strings.Cut(message, ": "); ok {
		if translated := s.trMemoryText(head); translated != head {
			return translated + ": " + s.trMemoryText(detail)
		}
	}
	return message
}
