package common

import (
	"bytes"
	"cmp"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"

	"github.com/0xsequence/authcontrol"
	"github.com/0xsequence/quotacontrol/proto"
)

func LoadConfig[T any](logger *slog.Logger) T {
	cfgFile := cmp.Or(os.Getenv("CONFIG_FILE"), "etc/config.json")
	logger.Debug("loading config", slog.String("file", cfgFile))

	rawCfg, err := os.ReadFile(cfgFile)
	if err != nil {
		logger.Error("cannot read config file", slog.Any("err", err))
		os.Exit(1)
	}

	var cfg T
	if err := json.Unmarshal(rawCfg, &cfg); err != nil {
		logger.Error("cannot load config", slog.Any("err", err))
		os.Exit(1)
	}
	return cfg
}

func SetProjectLimit(ctx context.Context, baseURL string, projectID uint64, limit any) error {
	body := bytes.Buffer{}
	if err := json.NewEncoder(&body).Encode(limit); err != nil {
		return err
	}

	req, err := http.NewRequest(http.MethodPost, baseURL+"/project/"+strconv.FormatUint(projectID, 10)+"/limit", &body)
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to set project limit: status %d", resp.StatusCode)
	}

	return nil
}

func ExecuteRequest(ctx context.Context, handler http.Handler, path, accessKey, jwt string) (int, http.Header, error) {
	req, err := http.NewRequest("POST", path, nil)
	if err != nil {
		return 0, nil, err
	}
	req.Header.Set("X-Real-IP", "127.0.0.1")
	if accessKey != "" {
		req.Header.Set(authcontrol.HeaderAccessKey, accessKey)
	}
	if jwt != "" {
		req.Header.Set("Authorization", "Bearer "+jwt)
	}

	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req.WithContext(ctx))

	status := rr.Result().StatusCode
	if status < http.StatusOK || status >= http.StatusBadRequest {
		w := proto.WebRPCError{}
		json.Unmarshal(rr.Body.Bytes(), &w)
		err = &w
	}

	return rr.Result().StatusCode, rr.Header(), err
}
