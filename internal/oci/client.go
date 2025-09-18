package oci

import (
	"io"
	"net/http"
	"net/url"
	"os"
)

type ocitransport struct {
	credentials Credentials
}

func (t ocitransport) RoundTrip(req *http.Request) (*http.Response, error) {
	t.credentials.ApplyTo(req)
	return http.DefaultTransport.RoundTrip(req)
}

type rwxtransport struct {
	token string
}

func (t rwxtransport) RoundTrip(req *http.Request) (*http.Response, error) {
	req.Header.Add("Authorization", "Bearer "+t.token)
	return http.DefaultTransport.RoundTrip(req)
}

type Client struct {
	http        http.Client
	registryURL url.URL
	repository  string
}

func NewClient(registryURL url.URL, repository string, credentials Credentials) *Client {
	return &Client{
		registryURL: registryURL,
		repository:  repository,
		http: http.Client{
			Transport: ocitransport{credentials: credentials},
		},
	}
}

func (c *Client) UploadLayer(l io.Reader) error {
	return NewLayerUpload(c.http, l, c.registryURL, c.repository).Upload()
}

func (c *Client) UploadManifest(layers []string) error {
	u, err := NewManifestUpload(
		c.http,
		http.Client{
			Transport: rwxtransport{
				token: os.Getenv("RWX_ACCESS_TOKEN"),
			},
		},
		c.registryURL,
		c.repository,
		layers,
	)
	if err != nil {
		return err
	}

	return u.Upload()
}
