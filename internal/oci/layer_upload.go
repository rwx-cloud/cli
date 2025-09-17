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

	"github.com/rwx-cloud/cli/internal/rwx"
	"golang.org/x/sync/errgroup"
)

type LayerUpload struct {
	http          http.Client
	layer         io.Reader
	registryURL   url.URL
	repository    string
	uploadURL     url.URL
	chunkSize     int
	uploadedSoFar int
	diffID        string
	digest        string
}

// Layer uploads are a one-time use struct to manage the state of an upload
func NewLayerUpload(c http.Client, layer io.Reader, registryURL url.URL, repository string) *LayerUpload {
	return &LayerUpload{
		http:          c,
		layer:         layer,
		registryURL:   registryURL,
		repository:    repository,
		chunkSize:     10 * 1024 * 1024, // 10 MiB, arbitrarily (we should experiment)
		uploadedSoFar: 0,
	}
}

func (u *LayerUpload) Upload() error {
	if err := u.startSession(); err != nil {
		return err
	}
	fmt.Println("session started")

	diffID := sha256.New()
	layer := io.TeeReader(u.layer, diffID)

	digest := sha256.New()
	gzipped, w := io.Pipe()

	eg := errgroup.Group{}
	eg.Go(func() error {
		defer w.Close()
		gz := gzip.NewWriter(io.MultiWriter(digest, w))
		defer gz.Close()

		_, err := io.Copy(gz, layer)
		return err
	})

	fmt.Println("chunks starting")

	for {
		buf := make([]byte, u.chunkSize)
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
		if err := u.uploadChunk(buf[:n]); err != nil {
			return fmt.Errorf("unable to upload layer chunk: %w", err)
		}
	}

	if err := eg.Wait(); err != nil {
		return fmt.Errorf("unable to gzip layer data: %w", err)
	}

	if err := u.closeSession(diffID, digest); err != nil {
		return fmt.Errorf("unable to close layer upload session: %w", err)
	}

	if rwx.IsRWX() {
		rwx.WriteValue("size-in-bytes", strconv.Itoa(u.uploadedSoFar))
		rwx.WriteValue("diff-id", u.diffID)
		rwx.WriteValue("digest", u.digest)
	} else {
		fmt.Printf("size-in-bytes=%v\n", u.uploadedSoFar)
		fmt.Printf("diff-id=%v\n", u.diffID)
		fmt.Printf("digest=%v\n", u.digest)
	}

	return nil
}

func (u *LayerUpload) startSession() error {
	req, err := http.NewRequest(http.MethodPost, u.registryURL.JoinPath("/v2/", u.repository, "/blobs/uploads/").String(), nil)
	if err != nil {
		return fmt.Errorf("unable to create upload request: %w", err)
	}
	req.Header.Set("Content-Length", "0")
	req.Header.Set("Content-Type", "application/octet-stream")

	resp, err := u.http.Do(req)
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
		if bytes > u.chunkSize {
			u.chunkSize = bytes
		}
	}

	if err := u.parseUploadURL(resp.Header.Get("Location")); err != nil {
		return err
	}

	return nil
}

func (u *LayerUpload) uploadChunk(chunk []byte) error {
	req, err := http.NewRequest(http.MethodPatch, u.uploadURL.String(), bytes.NewReader(chunk))
	if err != nil {
		return fmt.Errorf("unable to create chunk upload request: %w", err)
	}
	req.Header.Set("Content-Length", strconv.Itoa(len(chunk)))
	req.Header.Set("Content-Range", fmt.Sprintf("%v-%v", u.uploadedSoFar, u.uploadedSoFar+len(chunk)-1)) // inclusive on both sides
	req.Header.Set("Content-Type", "application/octet-stream")

	fmt.Printf("Uploading bytes %v-%v\n", u.uploadedSoFar, u.uploadedSoFar+len(chunk)-1)

	resp, err := u.http.Do(req)
	if err != nil {
		return fmt.Errorf("unable to upload layer chunk: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusAccepted && resp.StatusCode != http.StatusCreated {
		return fmt.Errorf("unexpected status code uploading layer chunk: %d", resp.StatusCode)
	}

	if err := u.parseUploadURL(resp.Header.Get("Location")); err != nil {
		return err
	}

	u.uploadedSoFar += len(chunk)
	return nil
}

func (u *LayerUpload) closeSession(diffID hash.Hash, digest hash.Hash) error {
	uncompressedSha := fmt.Sprintf("sha256:%x", diffID.Sum(nil))
	compressedSha := fmt.Sprintf("sha256:%x", digest.Sum(nil))
	fmt.Printf("Closing upload session with diffID %q, digest %q\n", uncompressedSha, compressedSha)
	u.diffID = uncompressedSha
	u.digest = compressedSha

	closeURL := u.uploadURL
	q := closeURL.Query()
	q.Set("digest", compressedSha)
	closeURL.RawQuery = q.Encode()

	req, err := http.NewRequest(http.MethodPut, closeURL.String(), nil)
	if err != nil {
		return fmt.Errorf("unable to create upload completion request: %w", err)
	}

	resp, err := u.http.Do(req)
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

func (u *LayerUpload) parseUploadURL(rawUploadURL string) error {
	if rawUploadURL == "" {
		return fmt.Errorf("missing Location header in upload initiation response")
	}

	uploadUrl, err := url.Parse(rawUploadURL)
	if err != nil {
		return fmt.Errorf("invalid upload URL %q: %w", rawUploadURL, err)
	}

	if uploadUrl.Host == "" {
		uploadUrl.Host = u.registryURL.Host
	}

	u.uploadURL = *uploadUrl

	return nil
}
