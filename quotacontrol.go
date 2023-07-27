package quotacontrol

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/0xsequence/quotacontrol/proto"
)

type LimitStore interface {
	GetAccessLimit(ctx context.Context, dappID uint64) ([]*proto.ServiceLimit, error)
}

type TokenStore interface {
	FindByTokenKey(ctx context.Context, tokenKey string) (*proto.AccessToken, error)
}

type UsageStore interface {
	GetAccountTotalUsage(ctx context.Context, dappID uint64, service proto.Service, min, max time.Time) (proto.AccessTokenUsage, error)
	UpdateTokenUsage(ctx context.Context, tokenKey string, service proto.Service, time time.Time, usage proto.AccessTokenUsage) error
}

func NewQuotaControl(cache CacheStorage, limit LimitStore, token TokenStore, usage UsageStore) proto.QuotaControl {
	return &quotaControl{
		cache:      cache,
		limitStore: limit,
		tokenStore: token,
		usageStore: usage,
	}
}

type quotaControl struct {
	cache      CacheStorage
	limitStore LimitStore
	tokenStore TokenStore
	usageStore UsageStore
}

func (q quotaControl) GetUsage(ctx context.Context, dappID uint64, service *proto.Service, now time.Time) (*proto.AccessTokenUsage, error) {
	min := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())
	max := min.AddDate(0, 1, -1)
	usage, err := q.usageStore.GetAccountTotalUsage(ctx, dappID, *service, min, max)
	if err != nil {
		return nil, err
	}
	return &usage, nil
}

func (q quotaControl) PrepareUsage(ctx context.Context, dappID uint64, service *proto.Service, now time.Time) (bool, error) {
	usage, err := q.GetUsage(ctx, dappID, service, now)
	if err != nil {
		return false, err
	}
	if err := q.cache.SetComputeUnits(ctx, service.GetQuotaKey(dappID, now), usage.GetTotalUsage()); err != nil {
		return false, err
	}
	return true, nil
}

func (q quotaControl) RetrieveToken(ctx context.Context, tokenKey string) (*proto.CachedToken, error) {
	token, err := q.tokenStore.FindByTokenKey(ctx, tokenKey)
	if err != nil {
		return nil, err
	}
	limit, err := q.limitStore.GetAccessLimit(ctx, token.ProjectID)
	if err != nil {
		return nil, err
	}
	record := proto.CachedToken{
		Config:      limit,
		AccessToken: token,
	}
	go q.cache.SetToken(ctx, &record)
	return &record, nil
}

func (q quotaControl) UpdateUsage(ctx context.Context, service *proto.Service, now time.Time, usage map[string]*proto.AccessTokenUsage) (map[string]bool, error) {
	var errs []error
	m := make(map[string]bool, len(usage))
	for tokenKey, tokenUsage := range usage {
		err := q.usageStore.UpdateTokenUsage(ctx, tokenKey, *service, now, *tokenUsage)
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

func (q quotaControl) GetAccessLimit(ctx context.Context, dappID uint64) ([]*proto.ServiceLimit, error) {
	return nil, proto.ErrNotImplemented
}

func (q quotaControl) SetAccessLimit(ctx context.Context, dappId uint64, config []*proto.ServiceLimit) (bool, error) {
	return false, proto.ErrNotImplemented
}

func (q quotaControl) GetAccessToken(ctx context.Context, tokenKey string) (*proto.AccessToken, error) {
	return nil, proto.ErrNotImplemented
}

func (q quotaControl) CreateAccessToken(ctx context.Context, dappID uint64, displayName string, allowedOrigins []string, allowedServices []*proto.Service) (*proto.AccessToken, error) {
	return nil, proto.ErrNotImplemented
}

func (q quotaControl) UpdateAccessToken(ctx context.Context, tokenKey string, displayName *string, allowedOrigins []string, allowedServices []*proto.Service) (*proto.AccessToken, error) {
	return nil, proto.ErrNotImplemented
}

func (q quotaControl) ListAccessTokens(ctx context.Context, dappID uint64) ([]*proto.AccessToken, error) {
	return nil, proto.ErrNotImplemented
}

func (q quotaControl) DisableAccessToken(ctx context.Context, tokenKey string) (bool, error) {
	return false, proto.ErrNotImplemented
}
