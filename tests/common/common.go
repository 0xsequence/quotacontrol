package common

import (
	"bytes"
	"cmp"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"time"
)

const httpTimeout = 5 * time.Second

type bearerTokenTransport struct {
	base  http.RoundTripper
	token string
}

func (t bearerTokenTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	req = req.Clone(req.Context())
	req.Header.Set("Authorization", "BEARER "+t.token)
	return t.base.RoundTrip(req)
}

// NewHTTPClient returns a timeout-bounded client that sends the quota-control token.
func NewHTTPClient(authToken string) *http.Client {
	return &http.Client{
		Timeout:   httpTimeout,
		Transport: bearerTokenTransport{base: http.DefaultTransport, token: authToken},
	}
}

func LoadConfig[T any]() (cfg T, err error) {
	cfgFile := cmp.Or(os.Getenv("CONFIG_FILE"), "etc/config.json")
	rawCfg, err := os.ReadFile(cfgFile)
	if err != nil {
		return cfg, fmt.Errorf("read config: %w", err)
	}
	if err := json.Unmarshal(rawCfg, &cfg); err != nil {
		return cfg, fmt.Errorf("decode config: %w", err)
	}
	return cfg, nil
}

func SetProjectLimit(ctx context.Context, httpClient *http.Client, baseURL string, projectID uint64, service string, limit any) error {
	body := bytes.Buffer{}
	if err := json.NewEncoder(&body).Encode(limit); err != nil {
		return fmt.Errorf("encode limit: %w", err)
	}

	url := baseURL + "/project/" + strconv.FormatUint(projectID, 10) + "/limit/" + service
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, &body)
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to set project limit: status %d", resp.StatusCode)
	}
	return nil
}

func ExecuteRequest(ctx context.Context, handler http.Handler, path string, headers http.Header) (int, http.Header, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, path, nil)
	if err != nil {
		return 0, nil, fmt.Errorf("create request: %w", err)
	}
	req.Header = headers.Clone()

	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req.WithContext(ctx))

	status := rr.Result().StatusCode
	if status < http.StatusOK || status >= http.StatusBadRequest {
		return status, rr.Header(), fmt.Errorf("request failed: status %d: %s", status, rr.Body.String())
	}
	return status, rr.Header(), nil
}
