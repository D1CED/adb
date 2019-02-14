package adb

import (
	"net"
	"regexp"
	"strconv"
	"strings"
)

func containsWhitespace(str string) bool {
	return strings.ContainsAny(str, " \t\v")
}

func isBlank(str string) bool {
	var whitespaceRegex = regexp.MustCompile(`^\s*$`)
	return whitespaceRegex.MatchString(str)
}

func getFreePort() (port int, err error) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return 0, err
	}
	defer listener.Close()

	addr := listener.Addr().String()
	_, portString, err := net.SplitHostPort(addr)
	if err != nil {
		return 0, err
	}
	return strconv.Atoi(portString)
}
