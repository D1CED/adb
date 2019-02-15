package adb

import (
	"net"
	"regexp"
	"strings"
)

func containsWhitespace(str string) bool {
	return strings.ContainsAny(str, " \t\v")
}

func isBlank(str string) bool {
	var whitespaceRegex = regexp.MustCompile(`^\s*$`)
	return whitespaceRegex.MatchString(str)
}

func getFreePort() int {
	addr, err := net.ResolveTCPAddr("tcp", "localhost:0")
	if err != nil {
		return 0
	}
	return addr.Port
}
