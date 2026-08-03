package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"time"

	"github.com/0xsequence/authcontrol"
	"github.com/go-chi/chi/v5"

	"github.com/0xsequence/quotacontrol"
	qcmw "github.com/0xsequence/quotacontrol/middleware"
	"github.com/0xsequence/quotacontrol/proto"
	"github.com/0xsequence/quotacontrol/tests/common"
)

var logger = slog.Default().With(slog.String("app", "quotacontrol-client"), slog.String("version", "v0.23"))

var (
	ProjectID uint64 = 1
	Service          = proto.Service_Indexer
)

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, os.Kill)
	defer cancel()

	cfg, err := common.LoadConfig[quotacontrol.Config]()
	if err != nil {
		logger.Error("failed to load config", slog.Any("err", err))
		os.Exit(1)
	}
	httpClient := common.NewHTTPClient(cfg.AuthToken)

	// Set up a per-service limit via the mock server HTTP endpoint.
	// The server expects the new Limit type, but the JSON fields are identical
	// to the old ServiceLimit (rateLimit, freeMax, overMax, etc.).
	limit := proto.ServiceLimit{RateLimit: 100, FreeMax: 1000, OverMax: 1000}
	if err := common.SetProjectLimit(ctx, httpClient, cfg.URL, ProjectID, Service.String(), limit); err != nil {
		logger.Error("failed to set project limit", slog.Any("err", err))
		os.Exit(1)
	}
	logger.Info("set project limit", slog.Uint64("projectID", ProjectID))

	baseClient := proto.NewQuotaControlClient(cfg.URL, httpClient)

	accessKey, err := baseClient.CreateAccessKey(ctx, ProjectID, "Test Key", false, nil, nil)
	if err != nil {
		logger.Error("failed to create access key", slog.Any("err", err))
		os.Exit(1)
	}
	logger.Info("created access key", slog.String("accessKey", accessKey.AccessKey))

	client := quotacontrol.NewClient(logger, Service, cfg, baseClient)
	go func() {
		if err := client.Run(ctx); err != nil {
			logger.Error("client run error", slog.Any("err", err))
			os.Exit(1)
		}
	}()

	limitCounter := quotacontrol.NewLimitCounter(Service, cfg.Redis, logger)

	r := chi.NewRouter()
	r.Use(authcontrol.VerifyToken(authcontrol.Options{}))
	r.Use(authcontrol.Session(authcontrol.Options{}))
	r.Use(qcmw.VerifyQuota(client, qcmw.Options{}))
	r.Use(qcmw.RateLimit(client, cfg.RateLimiter, limitCounter, qcmw.Options{}))
	r.Use(qcmw.SpendUsage(client, qcmw.Options{}))
	r.Post("/*", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	// Record usage before requests
	usageBefore, err := baseClient.GetUsage(ctx, ProjectID, nil, nil, nil, nil)
	if err != nil {
		logger.Error("failed to get usage", slog.Any("err", err))
		os.Exit(1)
	}

	// Test legacy middleware path: send requests through the full chain
	count := int64(10)
	logger.Info("testing legacy middleware path", slog.Int64("count", count))
	headers := http.Header{
		"X-Real-IP":                 {"127.0.0.1"},
		authcontrol.HeaderAccessKey: {accessKey.AccessKey},
	}
	for i := int64(0); i < count; i++ {
		status, _, reqErr := common.ExecuteRequest(ctx, r, "/", headers)
		if reqErr != nil {
			logger.Error("request error", slog.Int64("request", i+1), slog.Any("err", reqErr))
			os.Exit(1)
		}
		if status != http.StatusOK {
			logger.Error("unexpected status", slog.Int64("request", i+1), slog.Int("status", status))
			os.Exit(1)
		}
	}
	logger.Info("legacy middleware path OK")

	// Stop client to flush usage
	stopCtx, stopCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer stopCancel()
	client.Stop(stopCtx)

	// Verify usage delta
	usageAfter, err := baseClient.GetUsage(ctx, ProjectID, nil, nil, nil, nil)
	if err != nil {
		logger.Error("failed to get usage", slog.Any("err", err))
		os.Exit(1)
	}
	delta := usageAfter - usageBefore
	if delta != count {
		logger.Error("usage mismatch", slog.Int64("expected", count), slog.Int64("actual", delta))
		os.Exit(1)
	}
	logger.Info("usage sync verified", slog.Int64("delta", delta))

	logger.Info("all tests passed")
}
