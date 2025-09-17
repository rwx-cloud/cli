package oci

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"

	"github.com/opencontainers/go-digest"
	specs "github.com/opencontainers/image-spec/specs-go/v1"
)

type transport struct {
	credentials Credentials
}

func (t transport) RoundTrip(req *http.Request) (*http.Response, error) {
	t.credentials.ApplyTo(req)
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
			Transport: transport{credentials: credentials},
		},
	}
}

func (c *Client) UploadLayer(l io.Reader) error {
	return NewLayerUpload(c.http, l, c.registryURL, c.repository).Upload()
}

func (c *Client) CommitImage(tags []string) error {
	imageDescriptor, err := c.uploadImageConfig()
	if err != nil {
		return fmt.Errorf("unable to upload image config: %w", err)
	}

	if err := c.uploadManifest(tags, imageDescriptor); err != nil {
		return fmt.Errorf("unable to upload manifest: %w", err)
	}

	return nil
}

func (c *Client) uploadImageConfig(diffIDs []digest.Digest) (specs.Descriptor, error) {
	imageCfg := specs.Image{
		Platform: specs.Platform{
			Architecture: "amd64",
			OS:           "linux",
		},
		Config: specs.ImageConfig{
			Cmd:        []string{"/bin/bash"},
			WorkingDir: "/var/mint-workspace",
			Env:        []string{},
		},
		RootFS: specs.RootFS{
			Type:    "layers",
			DiffIDs: diffIDs,
		},
	}
	cfgBytes, err := json.Marshal(imageCfg)
	if err != nil {
		return specs.Descriptor{}, fmt.Errorf("unable to marshal image config: %w", err)
	}

	digest := digest.FromBytes(cfgBytes)

	url := c.registryURL.JoinPath("/v2/", c.repository, "/manifests/", digest.String())
	req, err := http.NewRequest(http.MethodPost, url.String(), bytes.NewReader(cfgBytes))
	if err != nil {
		return specs.Descriptor{}, fmt.Errorf("unable to create manifest upload request: %w", err)
	}
	req.Header.Set("Content-Length", fmt.Sprintf("%d", len(cfgBytes)))
	req.Header.Set("Content-Type", specs.MediaTypeImageConfig)

	resp, err := c.http.Do(req)
	if err != nil {
		return specs.Descriptor{}, fmt.Errorf("unable to upload manifest: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		return specs.Descriptor{}, fmt.Errorf("unexpected status code uploading manifest: %d", resp.StatusCode)
	}

	return specs.Descriptor{
		MediaType: specs.MediaTypeImageConfig,
		Digest:    digest,
		Size:      int64(len(bytes)),
	}, nil
}

func (c *Client) uploadManifest(tags []string, cfg specs.Descriptor) error {
	manifest := specs.Manifest{
		Config: cfg,
		Layers: []specs.Descriptor{},
	}
	bytes, err := json.Marshal(manifest)
	if err != nil {
		return fmt.Errorf("unable to marshal manifest: %w", err)
	}

	return nil
}
