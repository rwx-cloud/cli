package versions

import (
	"net/http"
)

const latestVersionHeader = "X-Rwx-Cli-Latest-Version"

type versionInterceptor struct {
	http.RoundTripper
}

func (vi versionInterceptor) RoundTrip(r *http.Request) (*http.Response, error) {
	resp, err := vi.RoundTripper.RoundTrip(r)
	if err == nil {
		if lv := resp.Header.Get(latestVersionHeader); lv != "" {
			_ = SetCliLatestVersion(lv)
		}
	}
	return resp, err
}

func NewRoundTripper(rt http.RoundTripper) http.RoundTripper {
	return versionInterceptor{RoundTripper: rt}
}
