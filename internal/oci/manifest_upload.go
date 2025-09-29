package oci

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"

	"github.com/opencontainers/go-digest"
	specsouter "github.com/opencontainers/image-spec/specs-go"
	specs "github.com/opencontainers/image-spec/specs-go/v1"
)

type ManifestUpload struct {
	ocihttp     http.Client
	rwxhttp     http.Client
	registryURL url.URL
	repository  string
	layers      []layer
	tags        []string
}

type layer struct {
	descriptor specs.Descriptor
	diffID     digest.Digest
}

type ociConfiguration struct {
	Architecture string   `json:"architecture"`
	OS           string   `json:"os"`
	Entrypoint   []string `json:"entrypoint"`
	Command      []string `json:"command"`
	WorkingDir   string   `json:"working_dir"`
	Env          []string `json:"env"`
}

// Manifest uploads are a one-time use struct to manage the state of an upload
func NewManifestUpload(ocihttp http.Client, rwxhttp http.Client, registryURL url.URL, repository string, layerStrings []string) (*ManifestUpload, error) {
	layers := make([]layer, 0, len(layerStrings))
	for _, layerString := range layerStrings {
		parts := strings.SplitN(layerString, "|", 3)
		if len(parts) != 3 {
			return nil, fmt.Errorf("invalid layer descriptor %q, expected <diff-id>|<digest>|<size-in-bytes>", layerString)
		}

		diffID, err := digest.Parse(parts[0])
		if err != nil {
			return nil, fmt.Errorf("invalid diff-id %q: %w", parts[0], err)
		}
		dgst, err := digest.Parse(parts[1])
		if err != nil {
			return nil, fmt.Errorf("invalid digest %q: %w", parts[1], err)
		}
		size, err := strconv.Atoi(parts[2])
		if err != nil {
			return nil, fmt.Errorf("invalid size in layer descriptor %q: %w", layerString, err)
		}

		layer := layer{
			descriptor: specs.Descriptor{
				MediaType: specs.MediaTypeImageLayerGzip,
				Digest:    dgst,
				Size:      int64(size),
			},
			diffID: diffID,
		}
		layers = append(layers, layer)
	}

	return &ManifestUpload{
		ocihttp:     ocihttp,
		rwxhttp:     rwxhttp,
		registryURL: registryURL,
		repository:  repository,
		layers:      layers,
		tags:        strings.Split(os.Getenv("OCI_TAGS"), ","),
	}, nil
}

func (u *ManifestUpload) Upload() error {
	ociCfg, err := u.fetchOCIConfiguration()
	if err != nil {
		return fmt.Errorf("unable to fetch OCI configuration: %w", err)
	}

	imgCfg, err := u.uploadImageConfig(ociCfg)
	if err != nil {
		return fmt.Errorf("unable to upload image config: %w", err)
	}

	manifest := u.constructManifest(imgCfg)

	for _, tag := range u.tags {
		if err := u.uploadManifest(manifest, tag); err != nil {
			return fmt.Errorf("unable to upload manifest for tag %q: %w", tag, err)
		}
	}

	return nil
}

func (u *ManifestUpload) fetchOCIConfiguration() (ociConfiguration, error) {
	configUrl := url.URL{}
	configUrl.Scheme = "https"
	configUrl.Host = os.Getenv("RWX_HOST")
	configUrl.Path = "/mint/api/unstable/images/pushes/configuration"

	req, err := http.NewRequest(http.MethodGet, configUrl.String(), nil)
	if err != nil {
		return ociConfiguration{}, fmt.Errorf("unable to create oci configuration request: %w", err)
	}
	req.Header.Set("X-RWX-Acknowledge-Unstable", "true")

	resp, err := u.rwxhttp.Do(req)
	if err != nil {
		return ociConfiguration{}, fmt.Errorf("unable to fetch oci configuration: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return ociConfiguration{}, fmt.Errorf("unexpected status code fetching oci configuration: %d", resp.StatusCode)
	}

	var ociCfg ociConfiguration
	if err := json.NewDecoder(resp.Body).Decode(&ociCfg); err != nil {
		return ociConfiguration{}, fmt.Errorf("unable to decode oci configuration: %w", err)
	}

	return ociCfg, nil
}

func (u *ManifestUpload) uploadImageConfig(ociCfg ociConfiguration) (specs.Descriptor, error) {
	diffIDs := make([]digest.Digest, 0, len(u.layers))
	for _, layer := range u.layers {
		diffIDs = append(diffIDs, layer.diffID)
	}

	imageCfg := specs.Image{
		Platform: specs.Platform{
			Architecture: ociCfg.Architecture,
			OS:           ociCfg.OS,
		},
		Config: specs.ImageConfig{
			Cmd:        ociCfg.Command,
			WorkingDir: ociCfg.WorkingDir,
			Env:        ociCfg.Env,
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

	url := u.registryURL.JoinPath("/v2/", u.repository, "/blobs/uploads/")
	q := url.Query()
	q.Add("digest", digest.String())
	url.RawQuery = q.Encode()
	req, err := http.NewRequest(http.MethodPost, url.String(), bytes.NewReader(cfgBytes))
	if err != nil {
		return specs.Descriptor{}, fmt.Errorf("unable to create manifest upload request: %w", err)
	}
	req.Header.Set("Content-Length", fmt.Sprintf("%d", len(cfgBytes)))
	req.Header.Set("Content-Type", "application/octet-stream")

	resp, err := u.ocihttp.Do(req)
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
		Size:      int64(len(cfgBytes)),
	}, nil
}

func (u *ManifestUpload) constructManifest(cfg specs.Descriptor) specs.Manifest {
	manifest := specs.Manifest{
		Versioned: specsouter.Versioned{
			SchemaVersion: 2,
		},
		MediaType: specs.MediaTypeImageManifest,
		Config:    cfg,
		Layers:    make([]specs.Descriptor, 0, len(u.layers)),
	}

	for _, layer := range u.layers {
		manifest.Layers = append(manifest.Layers, layer.descriptor)
	}

	return manifest
}

func (u *ManifestUpload) uploadManifest(manifest specs.Manifest, tag string) error {
	manifestBytes, err := json.Marshal(manifest)
	if err != nil {
		return fmt.Errorf("unable to marshal manifest: %w", err)
	}

	url := u.registryURL.JoinPath("/v2/", u.repository, "/manifests/", tag)
	req, err := http.NewRequest(http.MethodPut, url.String(), bytes.NewReader(manifestBytes))
	if err != nil {
		return fmt.Errorf("unable to create manifest upload request: %w", err)
	}
	req.Header.Set("Content-Length", fmt.Sprintf("%d", len(manifestBytes)))
	req.Header.Set("Content-Type", specs.MediaTypeImageManifest)

	resp, err := u.ocihttp.Do(req)
	if err != nil {
		return fmt.Errorf("unable to upload manifest: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		return fmt.Errorf("unexpected status code uploading manifest: %d", resp.StatusCode)
	}

	return nil
}
