package eval

import (
	"crypto/rand"
	"fmt"
	"os"
	"strings"
	"time"
	"unicode/utf8"
)

// ExpandMessage expands template variables in a deny/warn message.
// Supported variables:
//   - {command} — the full command string (sanitized, max 600 chars)
//   - {cmd} — the command name only
//   - {args} — the arguments (space-joined)
//   - {id} — unique ID (for temp file naming)
//   - {timestamp} — current timestamp
//   - {cwd} — $CLAUDE_PROJECT_DIR or current directory
func ExpandMessage(msg string, cmdName string, cmdArgs []string, fullCommand string) string {
	if !strings.Contains(msg, "{") {
		return msg
	}

	replacer := strings.NewReplacer(
		"{command}", sanitizeForMessage(fullCommand),
		"{cmd}", sanitizeForMessage(cmdName),
		"{args}", sanitizeForMessage(strings.Join(cmdArgs, " ")),
		"{id}", generateID(),
		"{timestamp}", time.Now().Format("20060102-150405"),
		"{cwd}", getProjectDir(),
	)

	return replacer.Replace(msg)
}

// maxSanitizedLen bounds interpolated values in deny/warn messages. 600 bytes
// leaves room for the degrade notice's approval procedure to remain readable
// after the user's own message (Plan 0022: the official docs state no limit
// on permissionDecisionReason, so this is a self-imposed sanity bound).
const maxSanitizedLen = 600

// sanitizeForMessage removes control characters and truncates long strings.
// Prevents prompt injection via command strings in deny messages.
func sanitizeForMessage(s string) string {
	var b strings.Builder
	for _, r := range s {
		if r < 0x20 && r != '\t' {
			b.WriteRune(' ')
		} else {
			b.WriteRune(r)
		}
	}
	result := b.String()
	if len(result) > maxSanitizedLen {
		// Cut on a rune boundary so multi-byte text can't produce broken UTF-8.
		cut := maxSanitizedLen
		for cut > 0 && !utf8.RuneStart(result[cut]) {
			cut--
		}
		result = result[:cut] + "..."
	}
	return result
}

// generateID generates a short unique ID for temp file naming.
func generateID() string {
	b := make([]byte, 6)
	if _, err := rand.Read(b); err != nil {
		return fmt.Sprintf("%d", time.Now().UnixNano())
	}
	return fmt.Sprintf("%x", b)
}

func getProjectDir() string {
	if dir := os.Getenv("CLAUDE_PROJECT_DIR"); dir != "" {
		return dir
	}
	if dir, err := os.Getwd(); err == nil {
		return dir
	}
	return "."
}
