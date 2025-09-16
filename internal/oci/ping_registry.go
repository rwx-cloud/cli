package oci

import (
	"fmt"
	"net/http"
	"net/url"
	"os"
)

func PingRegistry(url url.URL) (Credentials, error) {
	client := http.Client{}
	res, err := client.Get(url.JoinPath("/v2/").String())
	if err != nil {
		return nil, err
	}

	if res.StatusCode != http.StatusUnauthorized {
		return nil, fmt.Errorf("expected the registry to require authentication, but got code: %q", res.StatusCode)
	}

	authenticate := res.Header.Get("WWW-Authenticate")
	if authenticate == "" {
		return nil, fmt.Errorf("expected the registry to require authentication, but got no WWW-Authenticate header")
	}

	header, err := parseWWWAuthenticate(authenticate)
	if err != nil {
		return nil, fmt.Errorf("failed to parse WWW-Authenticate header: %w", err)
	}

	switch header.Scheme {
	case SchemeBasic:
		return NewBasicCredentials(os.Getenv("OCI_USERNAME"), os.Getenv("OCI_PASSWORD"))
	case SchemeBearer:
		return NewBearerCredentials(header)
	default:
		return nil, fmt.Errorf("unsupported authentication scheme %q in WWW-Authenticate header", header.Scheme)
	}
}
