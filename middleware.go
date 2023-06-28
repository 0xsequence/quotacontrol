package quotacontrol

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"sync/atomic"
	"time"

	"github.com/rs/zerolog"

	"github.com/0xsequence/quotacontrol/proto"
)

var (
	ErrTokenNotFound  = proto.Errorf(proto.ErrNotFound, "token not found")
	ErrInvalidOrigin  = proto.Errorf(proto.ErrPermissionDenied, "invalid origin")
	ErrInvalidService = proto.Errorf(proto.ErrPermissionDenied, "invalid service")
	ErrLimitExceeded  = proto.Errorf(proto.ErrResourceExhausted, "limit exceeded")
	ErrTimeout        = proto.Errorf(proto.ErrDeadlineExceeded, "timeout")
)

const (
	HeaderSequenceTokenKey     = "X-Sequence-Token-Key"
	HeaderSequenceComputeUnits = "X-Sequence-Compute-Units"
)

func NewMiddleware(log zerolog.Logger, s *proto.Service, cache CacheStorage, qc proto.QuotaControl, rl RateLimiter) *Middleware {
	return &Middleware{
		service: s,
		usage: &usageTracker{
			Usage: make(map[time.Time]map[string]*proto.AccessTokenUsage),
		},
		cache:       cache,
		quotaClient: qc,
		rateLimiter: rl,
		Log:         log,
	}
}

type Middleware struct {
	service     *proto.Service
	usage       *usageTracker
	cache       CacheStorage
	quotaClient proto.QuotaControl
	rateLimiter RateLimiter

	running int32
	ticker  *time.Ticker
	Log     zerolog.Logger
}

func (m *Middleware) Wrap(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ok, err := m.UseToken(r.Context(), r.Header.Get("X-Sequence-Token-Key"), r.Header.Get("Origin"))
		if err != nil {
			proto.RespondWithError(w, err)
			return
		}
		if !ok {
			proto.RespondWithError(w, ErrLimitExceeded)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (m *Middleware) UseToken(ctx context.Context, tokenKey, origin string) (bool, error) {
	now := GetTime(ctx)
	computeUnits := int64(1)
	if v, ok := ctx.Value(ctxKeyComputeUnits).(int64); ok {
		computeUnits = int64(v)
	}
	// fetch token
	token, err := m.cache.GetToken(ctx, tokenKey)
	if err != nil {
		if !errors.Is(err, ErrTokenNotFound) {
			return false, err
		}
		if token, err = m.quotaClient.RetrieveToken(ctx, tokenKey); err != nil {
			return false, err
		}
	}
	// validate token
	cfg, err := m.validateToken(token, origin)
	if err != nil {
		return false, err
	}
	key := m.service.GetQuotaKey(token.AccessToken.DappID, now)
	// check rate limit
	result, err := m.rateLimiter.RateLimit(ctx, key, int(computeUnits), RateLimit{Rate: cfg.ComputeRateLimit, Period: time.Hour})
	if err != nil {
		return false, err
	}
	if result.Allowed == 0 {
		return false, ErrLimitExceeded
	}
	// spend compute units
	for i := time.Duration(0); i < 3; i++ {
		resp, err := m.cache.SpendComputeUnits(ctx, key, computeUnits, cfg.ComputeMonthlyQuota)
		if err != nil {
			return false, err
		}
		switch *resp {
		case ALLOWED:
			m.usage.AddUsage(tokenKey, now, proto.AccessTokenUsage{ValidCompute: computeUnits})
			return true, nil
		case LIMITED:
			m.usage.AddUsage(tokenKey, now, proto.AccessTokenUsage{LimitedCompute: computeUnits})
			return false, ErrLimitExceeded
		case PING_BUILDER:
			ok, err := m.quotaClient.PrepareUsage(ctx, token.AccessToken.DappID, m.service, now)
			if err != nil {
				return false, err
			}
			if !ok {
				return false, ErrTimeout
			}
			fallthrough
		case WAIT_AND_RETRY:
			time.Sleep(time.Millisecond * 100 * (i + 1))
		}
	}
	return false, ErrTimeout
}

func (m *Middleware) validateToken(token *proto.CachedToken, origin string) (cfg *proto.ServiceLimit, err error) {
	if !token.AccessLimit.Active || !token.AccessToken.Active {
		return nil, ErrTokenNotFound
	}
	if len(token.AccessToken.AllowedOrigins) > 0 {
		if !token.AccessToken.ValidateOrigin(origin) {
			return nil, ErrInvalidOrigin
		}
	}
	cfg, ok := token.AccessLimit.Config[*m.service]
	if !ok {
		return nil, ErrInvalidService
	}
	return cfg, nil
}

func (m *Middleware) Run(ctx context.Context, updateFreq time.Duration) error {
	if m.IsRunning() {
		return fmt.Errorf("quota control: already running")
	}

	m.Log.Info().Str("op", "run").Msg("-> quota control: running")

	atomic.StoreInt32(&m.running, 1)
	m.ticker = time.NewTicker(updateFreq)
	defer atomic.StoreInt32(&m.running, 0)

	// Handle stop signal to ensure clean shutdown
	go func() {
		<-ctx.Done()
		m.Stop(context.Background())
	}()

	// Start the http server and serve!
	for range m.ticker.C {
		if err := m.usage.SyncUsage(ctx, m.quotaClient, m.service); err != nil {
			m.Log.Error().Err(err).Str("op", "run").Msg("-> quota control: failed to sync usage")
		}
	}
	return nil
}

func (m *Middleware) Stop(timeoutCtx context.Context) {
	if !m.IsRunning() || m.IsStopping() {
		return
	}
	atomic.StoreInt32(&m.running, 2)

	m.Log.Info().Str("op", "stop").Msg("-> quota control: stopping..")
	m.ticker.Stop()
	if err := m.usage.SyncUsage(timeoutCtx, m.quotaClient, m.service); err != nil {
		m.Log.Error().Err(err).Str("op", "run").Msg("-> quota control: failed to sync usage")
	}
	m.Log.Info().Str("op", "stop").Msg("-> quota control: stopped.")
}

func (s *Middleware) IsRunning() bool {
	return atomic.LoadInt32(&s.running) == 1
}

func (s *Middleware) IsStopping() bool {
	return atomic.LoadInt32(&s.running) == 2
}
