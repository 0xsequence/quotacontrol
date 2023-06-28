package quotacontrol

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/0xsequence/quotacontrol/proto"
)

type TokenStore interface {
	RetrieveToken(ctx context.Context, tokenKey string) (*proto.CachedToken, error)
}

type UsageStore interface {
	GetAccountTotalUsage(ctx context.Context, dappID uint64, service proto.Service, min, max time.Time) (proto.AccessTokenUsage, error)
	UpdateTokenUsage(ctx context.Context, tokenKey string, service proto.Service, time time.Time, usage proto.AccessTokenUsage) error
}

func NewQuotaControl(cache CacheStorage, token TokenStore, usage UsageStore) proto.QuotaControl {
	return &quotaControl{
		cache:      cache,
		tokenStore: token,
		usageStore: usage,
	}
}

type quotaControl struct {
	cache      CacheStorage
	tokenStore TokenStore
	usageStore UsageStore
}

func (q quotaControl) PrepareUsage(ctx context.Context, dappID uint64, service *proto.Service, now time.Time) (bool, error) {
	min := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())
	max := min.AddDate(0, 1, -1)
	usage, err := q.usageStore.GetAccountTotalUsage(ctx, dappID, *service, min, max)
	if err != nil {
		return false, err
	}
	if err := q.cache.SetComputeUnits(ctx, service.GetQuotaKey(dappID, now), usage.LimitedCompute+usage.ValidCompute); err != nil {
		return false, err
	}
	return true, nil
}

func (q quotaControl) RetrieveToken(ctx context.Context, tokenKey string) (*proto.CachedToken, error) {
	token, err := q.tokenStore.RetrieveToken(ctx, tokenKey)
	if err != nil {
		return nil, err
	}
	go q.cache.SetToken(ctx, token)
	return token, nil
}

func (q quotaControl) UpdateUsage(ctx context.Context, service *proto.Service, now time.Time, usage map[string]*proto.AccessTokenUsage) (map[string]bool, error) {
	var errs []error
	m := make(map[string]bool, len(usage))
	for tokenKey, tokenUsage := range usage {
		err := q.usageStore.UpdateTokenUsage(ctx, tokenKey, *service, time.Now(), *tokenUsage)
		if err != nil {
			errs = append(errs, fmt.Errorf("%s: %w", tokenKey, err))
		}
		m[tokenKey] = err == nil
	}
	if len(errs) > 0 {
		return m, errors.Join(errs...)
	}
	return m, nil
}
