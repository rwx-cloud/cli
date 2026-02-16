package docs

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/rwx-cloud/cli/cmd/rwx/config"
)

type Client struct {
	Host   string // default "www.rwx.com"
	Scheme string // default "https"
}

type SearchResponse struct {
	Query     string         `json:"query"`
	TotalHits int            `json:"totalHits"`
	Results   []SearchResult `json:"results"`
}

type SearchResult struct {
	ObjectID string `json:"objectID"`
	URL      string `json:"url"`
	Path     string `json:"path"`
	Title    string `json:"title"`
	Snippet  string `json:"snippet"`
	Body     string `json:"body"`
}

func (c Client) host() string {
	if c.Host != "" {
		return c.Host
	}
	return "www.rwx.com"
}

func (c Client) scheme() string {
	if c.Scheme != "" {
		return c.Scheme
	}
	return "https"
}

func (c Client) Search(query string, limit int) (*SearchResponse, error) {
	params := url.Values{}
	params.Set("q", query)
	if limit > 0 {
		params.Set("limit", strconv.Itoa(limit))
	}

	reqURL := fmt.Sprintf("%s://%s/docs/api/search?%s", c.scheme(), c.host(), params.Encode())

	req, err := http.NewRequest("GET", reqURL, nil)
	if err != nil {
		return nil, fmt.Errorf("unable to create request: %w", err)
	}
	req.Header.Set("User-Agent", fmt.Sprintf("rwx-cli/%s", config.Version))

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("unable to search docs: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		if resp.StatusCode == http.StatusNotFound {
			return nil, fmt.Errorf("docs search returned status 404: no results found")
		}
		return nil, fmt.Errorf("docs search returned status %d", resp.StatusCode)
	}

	var result SearchResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("unable to parse search response: %w", err)
	}

	return &result, nil
}

// resolveURL converts a URL or path into a fully qualified request URL.
// It accepts full URLs (with or without scheme), host-prefixed paths, and bare paths.
func (c Client) resolveURL(urlOrPath string) string {
	host := c.host()
	hostWithoutWWW := strings.TrimPrefix(host, "www.")

	// Full URL with scheme: https://www.rwx.com/docs/... or http://rwx.com/docs/...
	if strings.HasPrefix(urlOrPath, "https://") || strings.HasPrefix(urlOrPath, "http://") {
		return urlOrPath
	}

	// Host-prefixed without scheme: www.rwx.com/docs/... or rwx.com/docs/...
	if strings.HasPrefix(urlOrPath, host) || strings.HasPrefix(urlOrPath, hostWithoutWWW) {
		return fmt.Sprintf("%s://%s", c.scheme(), urlOrPath)
	}

	// Bare path: /docs/rwx/guides/ci
	return fmt.Sprintf("%s://%s%s", c.scheme(), host, urlOrPath)
}

func (c Client) FetchArticle(urlOrPath string) (string, error) {
	reqURL := c.resolveURL(urlOrPath)

	req, err := http.NewRequest("GET", reqURL, nil)
	if err != nil {
		return "", fmt.Errorf("unable to create request: %w", err)
	}
	req.Header.Set("Accept", "text/markdown")
	req.Header.Set("User-Agent", fmt.Sprintf("rwx-cli/%s", config.Version))

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("unable to fetch article: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		if resp.StatusCode == http.StatusNotFound {
			return "", fmt.Errorf("article not found: %s", urlOrPath)
		}
		return "", fmt.Errorf("docs article returned status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("unable to read article response: %w", err)
	}

	return string(body), nil
}
