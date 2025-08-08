package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/rwx-cloud/cli/internal/errors"
)

type McpTextResult struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

type McpGetRunTestFailuresRequest struct {
	RunUrls []string `json:"run_urls"`
}

func (c Client) McpGetRunTestFailures(cfg McpGetRunTestFailuresRequest) (*McpTextResult, error) {
	return c.makeMcpTextRequest("/mint/api/mcp/get_run_test_failures", cfg)
}

func (c Client) makeMcpTextRequest(endpoint string, body any) (*McpTextResult, error) {
	result := &McpTextResult{}

	encodedBody, err := json.Marshal(body)
	if err != nil {
		return result, errors.Wrap(err, "unable to encode as JSON")
	}

	req, err := http.NewRequest(http.MethodPost, endpoint, bytes.NewBuffer(encodedBody))
	if err != nil {
		return result, errors.Wrap(err, "unable to create new HTTP request")
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	resp, err := c.RoundTrip(req)
	if err != nil {
		return result, errors.Wrap(err, "HTTP request failed")
	}
	defer resp.Body.Close()

	if err = decodeMcpTextResult(resp, result); err != nil {
		return result, err
	}

	return result, nil
}

func decodeMcpTextResult(resp *http.Response, result *McpTextResult) error {
	if err := decodeResponseJSON(resp, &result); err != nil {
		return err
	}

	if result.Type != "text" {
		return fmt.Errorf("unexpected response type %q", result.Type)
	}

	return nil
}
