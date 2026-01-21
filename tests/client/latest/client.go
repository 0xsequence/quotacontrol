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

var logger = slog.Default().With(
	slog.String("app", "quotacontrol-client"),
	slog.String("version", "latest"),
)

var (
	ProjectID  uint64 = 1
	Service           = proto.Service_Indexer
	ServerAddr        = "localhost:8080"
)

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, os.Kill)
	defer cancel()

	cfg := common.LoadConfig[quotacontrol.Config](logger)

	limit := proto.Limit{
		ServiceLimit: map[string]proto.ServiceLimit{
			Service.String(): {
				RateLimit: 100,
				FreeMax:   1000,
				OverMax:   1000,
			},
		},
	}
	if err := common.SetProjectLimit(ctx, cfg.URL, ProjectID, limit); err != nil {
		logger.Error("failed to set project limit", slog.Any("err", err))
		os.Exit(1)
	}
	logger.Info("successfully set project limit", slog.Uint64("projectID", ProjectID))

	baseClient := proto.NewQuotaControlClient(cfg.URL, http.DefaultClient)

	logger.Info("server URL", slog.String("url", cfg.URL))

	accessKey, err := baseClient.CreateAccessKey(ctx, ProjectID, "Test Key", false, nil, nil)
	if err != nil {
		logger.Error("failed to create access key", slog.Any("err", err))
		os.Exit(1)
	}
	logger.Info("successfully created access key", slog.String("accessKeyID", accessKey.AccessKey))

	client := quotacontrol.NewClient(logger, Service, cfg, nil)
	go func() {
		if err := client.Run(ctx); err != nil {
			logger.Error("client run error", slog.Any("err", err))
			os.Exit(1)
		}
	}()

	counter := quotacontrol.NewLimitCounter(Service, cfg.Redis, logger)

	quotaOptions := qcmw.Options{}
	authOptions := authcontrol.Options{}

	r := chi.NewRouter()
	r.Use(authcontrol.VerifyToken(authOptions))
	r.Use(authcontrol.Session(authOptions))
	r.Use(qcmw.VerifyQuota(client, quotaOptions))
	r.Use(qcmw.RateLimit(client, cfg.RateLimiter, counter, quotaOptions))
	r.Use(qcmw.SpendUsage(client, quotaOptions))

	r.Post("/*", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("Hello, QuotaControl!"))
	})

	usageBefore, err := baseClient.GetUsage(ctx, ProjectID, nil, nil, nil, nil)
	if err != nil {
		logger.Error("failed to get usage", slog.Any("err", err))
		os.Exit(1)
	}

	count := int64(10)
	logger.Info("sending requests", slog.Int64("count", count))
	for i := int64(0); i < count; i++ {
		logger := logger.With(slog.Int64("requestNumber", i+1))
		status, _, err := common.ExecuteRequest(ctx, r, "/", accessKey.AccessKey, "")
		if err != nil {
			logger.Error("request error", slog.Any("err", err))
		}
		if status != http.StatusOK {
			logger.Error("unexpected status code", slog.Int("status", status))
		}
	}

	logger.Info("shutting down client")

	ctxShutDown, cancelShutDown := context.WithTimeout(context.Background(), time.Second*5)
	defer cancelShutDown()
	client.Stop(ctxShutDown)

	logger.Info("client shut down successfully")

	usageAfter, err := baseClient.GetUsage(ctx, ProjectID, nil, nil, nil, nil)
	if err != nil {
		logger.Error("failed to get usage", slog.Any("err", err))
		os.Exit(1)
	}
	logger.Info("usage before and after requests", slog.Any("before", usageBefore), slog.Any("after", usageAfter))
	if usageAfter-usageBefore != int64(count) {
		logger.Error("usage did not increase as expected", slog.Int64("expectedIncrease", count), slog.Int64("actualIncrease", usageAfter-usageBefore))
		os.Exit(1)
	}
	logger.Info("usage increased as expected", slog.Int64("increase", usageAfter-usageBefore))
}
