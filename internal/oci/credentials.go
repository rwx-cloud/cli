package oci

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
)

type Credentials interface {
	ApplyTo(request *http.Request)
}

type BasicCredentials struct {
	username string
	password string
}

func NewBasicCredentials(username, password string) (BasicCredentials, error) {
	if username == "" || password == "" {
		return BasicCredentials{}, fmt.Errorf("both username and password must be provided for basic authentication")
	}

	return BasicCredentials{username: username, password: password}, nil
}

func (c BasicCredentials) ApplyTo(request *http.Request) {
	token := base64.StdEncoding.EncodeToString([]byte(c.username + ":" + c.password))
	request.Header.Add("Authorization", "Basic "+token)
}

type BearerCredentials struct {
	token string
}

func NewBearerCredentials(header WWWAuthenticateHeader) (BearerCredentials, error) {
	realm := header.Params["realm"]
	req, err := http.NewRequest(http.MethodGet, realm, nil)
	if err != nil {
		return BearerCredentials{}, fmt.Errorf("failed to create request to get bearer token: %w", err)
	}

	for key, value := range header.Params {
		if key != "realm" {
			q := req.URL.Query()
			q.Add(key, value)
			req.URL.RawQuery = q.Encode()
		}
	}

	client := http.Client{}
	res, err := client.Do(req)
	if err != nil {
		return BearerCredentials{}, fmt.Errorf("failed to get bearer token: %w", err)
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		return BearerCredentials{}, fmt.Errorf("failed to get bearer token, status code: %d", res.StatusCode)
	}

	var tokenResponse struct {
		Token string `json:"token"`
	}
	if err := json.NewDecoder(res.Body).Decode(&tokenResponse); err != nil {
		return BearerCredentials{}, fmt.Errorf("failed to decode bearer token response: %w", err)
	}

	token := tokenResponse.Token
	if token == "" {
		return BearerCredentials{}, fmt.Errorf("received empty bearer token")
	}

	return BearerCredentials{token: token}, nil
}

func (c BearerCredentials) ApplyTo(request *http.Request) {
	request.Header.Add("Authorization", "Bearer "+c.token)
}
