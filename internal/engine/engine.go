package engine

import (
	"os"
	"strings"
)

func Cmd() string {
	if eng := strings.TrimSpace(os.Getenv("CODERAFT_ENGINE")); eng != "" {
		return eng
	}
	return "docker"
}
