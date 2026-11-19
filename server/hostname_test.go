package server

import (
	"regexp"
	"testing"
)

func TestGenerateHostnameFormat(t *testing.T) {
	old := hostnameSuffix
	hostnameSuffix = ".tunnel.example.com"
	defer func() { hostnameSuffix = old }()

	pattern := regexp.MustCompile(`^[a-z]+-[a-z]+-\d{1,4}\.tunnel\.example\.com$`)
	for i := 0; i < 200; i++ {
		if h := GenerateHostname(); !pattern.MatchString(h) {
			t.Fatalf("hostname %q does not match %s", h, pattern)
		}
	}
}
