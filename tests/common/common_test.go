package common

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestNewHTTPClient(t *testing.T) {
	authorization := make(chan string, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authorization <- r.Header.Get("Authorization")
		w.WriteHeader(http.StatusNoContent)
	}))
	t.Cleanup(server.Close)

	client := NewHTTPClient("secret")
	resp, err := client.Get(server.URL)
	if err != nil {
		t.Fatal(err)
	}
	if err := resp.Body.Close(); err != nil {
		t.Fatal(err)
	}
	if client.Timeout != httpTimeout {
		t.Fatalf("expected timeout %s, got %s", httpTimeout, client.Timeout)
	}
	if got := <-authorization; got != "BEARER secret" {
		t.Fatalf("unexpected authorization header %q", got)
	}
}

func TestLoadConfig(t *testing.T) {
	t.Run("loads configured file", func(t *testing.T) {
		cfgFile := filepath.Join(t.TempDir(), "config.json")
		if err := os.WriteFile(cfgFile, []byte(`{"name":"quota"}`), 0o600); err != nil {
			t.Fatal(err)
		}
		t.Setenv("CONFIG_FILE", cfgFile)

		cfg, err := LoadConfig[struct {
			Name string `json:"name"`
		}]()
		if err != nil {
			t.Fatal(err)
		}
		if cfg.Name != "quota" {
			t.Fatalf("expected quota, got %q", cfg.Name)
		}
	})

	t.Run("returns read error", func(t *testing.T) {
		t.Setenv("CONFIG_FILE", filepath.Join(t.TempDir(), "missing.json"))
		if _, err := LoadConfig[struct{}](); err == nil || !strings.Contains(err.Error(), "read config") {
			t.Fatalf("expected read error, got %v", err)
		}
	})

	t.Run("returns decode error", func(t *testing.T) {
		cfgFile := filepath.Join(t.TempDir(), "config.json")
		if err := os.WriteFile(cfgFile, []byte(`{`), 0o600); err != nil {
			t.Fatal(err)
		}
		t.Setenv("CONFIG_FILE", cfgFile)
		if _, err := LoadConfig[struct{}](); err == nil || !strings.Contains(err.Error(), "decode config") {
			t.Fatalf("expected decode error, got %v", err)
		}
	})
}

func TestSetProjectLimit(t *testing.T) {
	t.Run("sends version-neutral payload", func(t *testing.T) {
		type request struct {
			method      string
			path        string
			contentType string
			limit       struct {
				FreeMax int64 `json:"freeMax"`
			}
		}
		requests := make(chan request, 1)
		server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			got := request{method: r.Method, path: r.URL.Path, contentType: r.Header.Get("Content-Type")}
			if err := json.NewDecoder(r.Body).Decode(&got.limit); err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			requests <- got
			w.WriteHeader(http.StatusOK)
		}))
		t.Cleanup(server.Close)
		httpClient := server.Client()
		httpClient.Timeout = time.Second

		if err := SetProjectLimit(context.Background(), httpClient, server.URL, 7, "indexer", struct {
			FreeMax int64 `json:"freeMax"`
		}{FreeMax: 100}); err != nil {
			t.Fatal(err)
		}
		got := <-requests
		if got.method != http.MethodPost {
			t.Fatalf("unexpected method %q", got.method)
		}
		if got.path != "/project/7/limit/indexer" {
			t.Fatalf("unexpected path %q", got.path)
		}
		if got.contentType != "application/json" {
			t.Fatalf("unexpected content type %q", got.contentType)
		}
		if got.limit.FreeMax != 100 {
			t.Fatalf("expected limit 100, got %d", got.limit.FreeMax)
		}
	})

	t.Run("returns encode error", func(t *testing.T) {
		err := SetProjectLimit(context.Background(), &http.Client{Timeout: time.Second}, "http://example.com", 7, "indexer", make(chan int))
		if err == nil || !strings.Contains(err.Error(), "encode limit") {
			t.Fatalf("expected encode error, got %v", err)
		}
	})

	t.Run("returns request error", func(t *testing.T) {
		err := SetProjectLimit(context.Background(), &http.Client{Timeout: time.Second}, "\x00", 7, "indexer", struct{}{})
		if err == nil || !strings.Contains(err.Error(), "create request") {
			t.Fatalf("expected request error, got %v", err)
		}
	})

	t.Run("returns client error", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		err := SetProjectLimit(ctx, &http.Client{Timeout: time.Second}, "http://example.com", 7, "indexer", struct{}{})
		if err == nil || !strings.Contains(err.Error(), "send request") {
			t.Fatalf("expected client error, got %v", err)
		}
	})

	t.Run("returns server error", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			http.Error(w, "no", http.StatusBadRequest)
		}))
		t.Cleanup(server.Close)
		httpClient := server.Client()
		httpClient.Timeout = time.Second
		if err := SetProjectLimit(context.Background(), httpClient, server.URL, 7, "indexer", struct{}{}); err == nil || !strings.Contains(err.Error(), "status 400") {
			t.Fatalf("expected status error, got %v", err)
		}
	})
}

func TestExecuteRequest(t *testing.T) {
	t.Run("forwards headers", func(t *testing.T) {
		handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Header.Get("X-Access-Key") != "key" {
				http.Error(w, "missing key", http.StatusUnauthorized)
				return
			}
			w.Header().Set("X-Test", "ok")
			w.WriteHeader(http.StatusNoContent)
		})
		status, headers, err := ExecuteRequest(context.Background(), handler, "/", http.Header{"X-Access-Key": {"key"}})
		if err != nil {
			t.Fatal(err)
		}
		if status != http.StatusNoContent || headers.Get("X-Test") != "ok" {
			t.Fatalf("unexpected response: status=%d header=%q", status, headers.Get("X-Test"))
		}
	})

	t.Run("returns handler error", func(t *testing.T) {
		handler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			http.Error(w, "failed", http.StatusInternalServerError)
		})
		status, _, err := ExecuteRequest(context.Background(), handler, "/", nil)
		if status != http.StatusInternalServerError || err == nil || !strings.Contains(err.Error(), "failed") {
			t.Fatalf("unexpected response: status=%d err=%v", status, err)
		}
	})

	t.Run("returns request error", func(t *testing.T) {
		status, headers, err := ExecuteRequest(context.Background(), http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}), "\x00", nil)
		if status != 0 || headers != nil || err == nil || !strings.Contains(err.Error(), "create request") {
			t.Fatalf("unexpected response: status=%d headers=%v err=%v", status, headers, err)
		}
	})
}
