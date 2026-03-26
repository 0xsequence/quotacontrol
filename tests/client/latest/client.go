package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"time"

	"github.com/0xsequence/authcontrol"
	"github.com/0xsequence/quotacontrol"
	qcmw "github.com/0xsequence/quotacontrol/middleware"
	"github.com/0xsequence/quotacontrol/proto"
	"github.com/0xsequence/quotacontrol/tests/common"
	"github.com/go-chi/chi/v5"
)

var logger = slog.Default().With(slog.String("app", "quotacontrol-client"), slog.String("version", "latest"))

var (
	ProjectID uint64 = 1
	Service          = proto.Service_Indexer
)

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, os.Kill)
	defer cancel()

	cfg := common.LoadConfig[quotacontrol.Config](logger)

	// Set up a per-service limit via the mock server HTTP endpoint
	limit := proto.Limit{RateLimit: 100, FreeMax: 1000, OverMax: 1000}
	if err := common.SetProjectLimit(ctx, cfg.URL, ProjectID, Service, limit); err != nil {
		logger.Error("failed to set project limit", slog.Any("err", err))
		os.Exit(1)
	}
	logger.Info("set project limit", slog.Uint64("projectID", ProjectID))

	baseClient := proto.NewQuotaControlClient(cfg.URL, http.DefaultClient)

	accessKey, err := baseClient.CreateAccessKey(ctx, ProjectID, "Test Key", false, nil, nil)
	if err != nil {
		logger.Error("failed to create access key", slog.Any("err", err))
		os.Exit(1)
	}
	logger.Info("created access key", slog.String("accessKey", accessKey.AccessKey))

	client := quotacontrol.NewClient(logger, Service, cfg, nil)
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

	// Test legacy middleware path: send requests through the full chain
	count := int64(10)
	logger.Info("testing legacy middleware path", slog.Int64("count", count))
	for i := int64(0); i < count; i++ {
		status, _, err := common.ExecuteRequest(ctx, r, "/", accessKey.AccessKey, "")
		if err != nil {
			logger.Error("request error", slog.Int64("request", i+1), slog.Any("err", err))
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

	// Verify usage was synced
	usageAfter, err := baseClient.GetUsage(ctx, ProjectID, nil, nil, nil, nil)
	if err != nil {
		logger.Error("failed to get usage", slog.Any("err", err))
		os.Exit(1)
	}
	if usageAfter != count {
		logger.Error("usage mismatch", slog.Int64("expected", count), slog.Int64("actual", usageAfter))
		os.Exit(1)
	}
	logger.Info("usage sync verified", slog.Int64("usage", usageAfter))

	// Test V2 paths: individual fetch methods
	logger.Info("testing V2 fetch paths")

	client2 := quotacontrol.NewClient(logger, Service, cfg, nil)

	info, err := client2.FetchProjectInfo(ctx, ProjectID)
	if err != nil {
		logger.Error("FetchProjectInfo failed", slog.Any("err", err))
		os.Exit(1)
	}
	if info.ID != ProjectID {
		logger.Error("FetchProjectInfo returned wrong project", slog.Uint64("expected", ProjectID), slog.Uint64("actual", info.ID))
		os.Exit(1)
	}
	logger.Info("FetchProjectInfo OK", slog.Uint64("projectID", info.ID))

	svcLimit, err := client2.FetchServiceLimit(ctx, ProjectID)
	if err != nil {
		logger.Error("FetchServiceLimit failed", slog.Any("err", err))
		os.Exit(1)
	}
	if svcLimit.FreeMax != limit.FreeMax {
		logger.Error("FetchServiceLimit returned wrong limit", slog.Int64("expected", limit.FreeMax), slog.Int64("actual", svcLimit.FreeMax))
		os.Exit(1)
	}
	logger.Info("FetchServiceLimit OK", slog.Int64("freeMax", svcLimit.FreeMax))

	ak, err := client2.FetchAccessKey(ctx, accessKey.AccessKey)
	if err != nil {
		logger.Error("FetchAccessKey failed", slog.Any("err", err))
		os.Exit(1)
	}
	if ak.AccessKey != accessKey.AccessKey {
		logger.Error("FetchAccessKey returned wrong key", slog.String("expected", accessKey.AccessKey), slog.String("actual", ak.AccessKey))
		os.Exit(1)
	}
	logger.Info("FetchAccessKey OK", slog.String("accessKey", ak.AccessKey))

	logger.Info("all tests passed")
}
