package oci

import (
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"fmt"
	"hash"
	"io"
	"net/http"
	"net/url"
	"strconv"

	"golang.org/x/sync/errgroup"
)

type transport struct {
	credentials Credentials
}

func (t transport) RoundTrip(req *http.Request) (*http.Response, error) {
	t.credentials.ApplyTo(req)
	return http.DefaultTransport.RoundTrip(req)
}

type Client struct {
	http          http.Client
	registryURL   url.URL
	repository    string
	uploadURL     url.URL
	chunkSize     int
	uploadedSoFar int
}

type ClientConfig struct {
	RegistryURL url.URL
	Repository  string
	Credentials Credentials
}

func NewClient(cfg ClientConfig) *Client {
	return &Client{
		registryURL:   cfg.RegistryURL,
		repository:    cfg.Repository,
		chunkSize:     10 * 1024 * 1024, // 10 MiB, arbitrarily (we should experiment)
		uploadedSoFar: 0,
		http: http.Client{
			Transport: transport{credentials: cfg.Credentials},
		},
	}
}

func (c *Client) UploadLayer(l io.Reader) error {
	if err := c.startSession(); err != nil {
		return err
	}
	fmt.Println("session started")

	hash := sha256.New()
	gzipped, w := io.Pipe()

	eg := errgroup.Group{}
	eg.Go(func() error {
		defer w.Close()
		gz := gzip.NewWriter(io.MultiWriter(hash, w))
		defer gz.Close()

		_, err := io.Copy(gz, l)
		return err
	})

	fmt.Println("chunks starting")

	for {
		buf := make([]byte, c.chunkSize)
		fmt.Println("reading chunk")
		n, err := io.ReadFull(gzipped, buf)
		fmt.Println("chunk read")
		if err != nil && err != io.ErrUnexpectedEOF && err != io.EOF && err != io.ErrShortBuffer {
			return fmt.Errorf("unable to read layer data: %w", err)
		}
		if n == 0 {
			break
		}
		fmt.Println("uploading chunk")
		if err := c.uploadChunk(buf[:n]); err != nil {
			return fmt.Errorf("unable to upload layer chunk: %w", err)
		}
	}

	if err := eg.Wait(); err != nil {
		return fmt.Errorf("unable to gzip layer data: %w", err)
	}

	if err := c.closeSession(hash); err != nil {
		return fmt.Errorf("unable to close layer upload session: %w", err)
	}

	return nil
}

func (c *Client) startSession() error {
	req, err := http.NewRequest(http.MethodPost, c.registryURL.JoinPath("/v2/", c.repository, "/blobs/uploads/").String(), nil)
	if err != nil {
		return fmt.Errorf("unable to create upload request: %w", err)
	}
	req.Header.Set("Content-Length", "0")
	req.Header.Set("Content-Type", "application/octet-stream")

	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("unable to initiate layer upload: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusAccepted {
		return fmt.Errorf("unexpected status code initiating layer upload: %d", resp.StatusCode)
	}

	minChunkSize := resp.Header.Get("OCI-Chunk-Min-Length")
	if minChunkSize != "" {
		bytes, err := strconv.Atoi(minChunkSize)
		if err != nil {
			return fmt.Errorf("invalid OCI-Chunk-Min-Length header %q: %w", minChunkSize, err)
		}
		if bytes > c.chunkSize {
			c.chunkSize = bytes
		}
	}

	if err := c.parseUploadURL(resp.Header.Get("Location")); err != nil {
		return err
	}

	return nil
}

func (c *Client) uploadChunk(chunk []byte) error {
	req, err := http.NewRequest(http.MethodPatch, c.uploadURL.String(), bytes.NewReader(chunk))
	if err != nil {
		return fmt.Errorf("unable to create chunk upload request: %w", err)
	}
	req.Header.Set("Content-Length", strconv.Itoa(len(chunk)))
	req.Header.Set("Content-Range", fmt.Sprintf("%v-%v", c.uploadedSoFar, c.uploadedSoFar+len(chunk)-1)) // inclusive on both sides
	req.Header.Set("Content-Type", "application/octet-stream")

	fmt.Printf("Uploading bytes %v-%v\n", c.uploadedSoFar, c.uploadedSoFar+len(chunk)-1)

	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("unable to upload layer chunk: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusAccepted && resp.StatusCode != http.StatusCreated {
		return fmt.Errorf("unexpected status code uploading layer chunk: %d", resp.StatusCode)
	}

	if err := c.parseUploadURL(resp.Header.Get("Location")); err != nil {
		return err
	}

	c.uploadedSoFar += len(chunk)
	return nil
}

func (c *Client) closeSession(h hash.Hash) error {
	compressedSha := fmt.Sprintf("sha256:%x", h.Sum(nil))
	fmt.Printf("Closing upload session with digest %s\n", compressedSha)

	closeURL := c.uploadURL
	q := closeURL.Query()
	q.Set("digest", compressedSha)
	closeURL.RawQuery = q.Encode()

	req, err := http.NewRequest(http.MethodPut, closeURL.String(), nil)
	if err != nil {
		return fmt.Errorf("unable to create upload completion request: %w", err)
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("unable to close layer upload: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		return fmt.Errorf("unexpected status code closing layer upload: %d", resp.StatusCode)
	}

	fmt.Printf("blob location: %q\n", resp.Header.Get("Location"))

	return nil
}

func (c *Client) parseUploadURL(rawUploadURL string) error {
	if rawUploadURL == "" {
		return fmt.Errorf("missing Location header in upload initiation response")
	}

	uploadUrl, err := url.Parse(rawUploadURL)
	if err != nil {
		return fmt.Errorf("invalid upload URL %q: %w", rawUploadURL, err)
	}

	if uploadUrl.Host == "" {
		uploadUrl.Host = c.registryURL.Host
	}

	c.uploadURL = *uploadUrl

	return nil
}
