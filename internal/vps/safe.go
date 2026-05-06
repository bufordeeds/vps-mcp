package vps

import (
	"fmt"
	"regexp"
	"strings"
)

// quoteForShell wraps a value in single quotes, refusing values that
// contain characters that would break out of single quotes.
//
// We do this rather than a generic escaping pass so that a typo in a
// regex or unexpected unicode never ends up running arbitrary commands
// on the VPS.
func quoteForShell(s string) (string, error) {
	if strings.ContainsAny(s, "'\\\n\r\x00") {
		return "", fmt.Errorf("argument contains forbidden character")
	}
	return "'" + s + "'", nil
}

// validDomain is conservative — letters, digits, hyphen, dot, no leading
// hyphen or dot. Good enough for a tool exposed to an LLM that can be
// nudged to send weird input.
var validDomain = regexp.MustCompile(`^[a-zA-Z0-9](?:[a-zA-Z0-9-]*[a-zA-Z0-9])?(?:\.[a-zA-Z0-9](?:[a-zA-Z0-9-]*[a-zA-Z0-9])?)+$`)

func checkDomain(d string) error {
	if !validDomain.MatchString(d) {
		return fmt.Errorf("invalid domain %q", d)
	}
	return nil
}

// validContainerName matches Docker's allowed name pattern.
var validContainerName = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9_.-]*$`)

func checkContainerName(n string) error {
	if !validContainerName.MatchString(n) {
		return fmt.Errorf("invalid container name %q", n)
	}
	return nil
}

// validAbsPath: absolute, no shell metacharacters, no .. segments.
func checkAbsPath(p string) error {
	if !strings.HasPrefix(p, "/") {
		return fmt.Errorf("path must be absolute")
	}
	if strings.ContainsAny(p, "*?[]{}();&|<>$`\"' \t\n") {
		return fmt.Errorf("path contains forbidden characters")
	}
	for _, seg := range strings.Split(p, "/") {
		if seg == ".." {
			return fmt.Errorf("path may not contain ..")
		}
	}
	return nil
}
