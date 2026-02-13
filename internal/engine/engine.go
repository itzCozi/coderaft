package engine

import (
	"os"
	"regexp"
	"strings"
)

var validEnginePattern = regexp.MustCompile(`^[a-zA-Z0-9_-]+$`)

func Cmd() string {
	if eng := strings.TrimSpace(os.Getenv("CODERAFT_ENGINE")); eng != "" {

		if validEnginePattern.MatchString(eng) {
			return eng
		}

	}
	return "docker"
}
