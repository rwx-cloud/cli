package versions

import (
	"net/http"
)

const latestVersionHeader = "X-Rwx-Cli-Latest-Version"

type versionInterceptor struct {
	http.RoundTripper
	backend Backend
}

func (vi versionInterceptor) RoundTrip(r *http.Request) (*http.Response, error) {
	resp, err := vi.RoundTripper.RoundTrip(r)
	if err == nil {
		if lv := resp.Header.Get(latestVersionHeader); lv != "" {
			_ = SetCliLatestVersion(lv)
			SaveLatestVersionToFile(vi.backend)
		}
	}
	return resp, err
}

func NewRoundTripper(rt http.RoundTripper, backend Backend) http.RoundTripper {
	return versionInterceptor{RoundTripper: rt, backend: backend}
}
