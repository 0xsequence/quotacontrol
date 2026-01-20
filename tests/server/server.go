package main

import (
	"context"
	_ "embed"
	"fmt"
	"log"
	"log/slog"
	"os"
	"os/signal"

	"github.com/0xsequence/quotacontrol"
	"github.com/0xsequence/quotacontrol/tests/common"
	"github.com/0xsequence/quotacontrol/tests/mock"
	"github.com/redis/go-redis/v9"
)

var logger = slog.Default().With(
	slog.String("app", "quotacontrol-server"),
	slog.String("version", "latest"),
)

func main() {
	cfg := common.LoadConfig[quotacontrol.Config](logger)

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, os.Kill)
	defer cancel()

	client := redis.NewClient(&redis.Options{
		Addr: fmt.Sprintf("%s:%d", cfg.Redis.Host, cfg.Redis.Port),
	})
	if err := client.Ping(ctx).Err(); err != nil {
		log.Fatalf("failed to connect to redis: %v", err)
	}

	_, cleanup := mock.NewServer(&cfg, &mock.Options{
		RedisClient: client,
		Logger:      logger,
	})
	defer cleanup()

	logger.Info("QuotaControl server running", slog.String("url", cfg.URL))

	<-ctx.Done()
	logger.Info("QuotaContol server shutting down...")
	cleanup()
}
