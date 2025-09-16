package oci

import (
	"fmt"
	"strings"
)

type WWWAuthenticateHeader struct {
	Scheme Scheme
	Params map[string]string
}

type Scheme string

const (
	SchemeBasic  Scheme = "Basic"
	SchemeBearer Scheme = "Bearer"
)

func parseWWWAuthenticate(header string) (WWWAuthenticateHeader, error) {
	parts := strings.SplitN(header, " ", 2)
	if len(parts) != 2 {
		return WWWAuthenticateHeader{}, fmt.Errorf("invalid WWW-Authenticate header, expected '<scheme> <params>' got %q", header)
	}

	var scheme Scheme
	switch strings.ToLower(parts[0]) {
	case strings.ToLower(string(SchemeBasic)):
		scheme = SchemeBasic
	case strings.ToLower(string(SchemeBearer)):
		scheme = SchemeBearer
	default:
		return WWWAuthenticateHeader{}, fmt.Errorf("unsupported authentication scheme %q in WWW-Authenticate header", parts[0])
	}

	params := make(map[string]string)
	for param := range strings.SplitSeq(parts[1], ",") {
		kv := strings.SplitN(strings.TrimSpace(param), "=", 2)
		if len(kv) != 2 {
			return WWWAuthenticateHeader{}, fmt.Errorf("invalid parameter %q in WWW-Authenticate header", param)
		}

		key := kv[0]
		value := strings.Trim(kv[1], `"`)
		params[key] = value
	}

	return WWWAuthenticateHeader{Scheme: scheme, Params: params}, nil
}
