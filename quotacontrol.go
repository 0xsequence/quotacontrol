package quotacontrol

import (
	"context"
	"crypto/rand"
	"encoding/binary"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/0xsequence/quotacontrol/proto"
	"github.com/jxskiss/base62"
)

type LimitStore interface {
	GetAccessLimit(ctx context.Context, projectID uint64) (*proto.Limit, error)
	SetAccessLimit(ctx context.Context, projectID uint64, config *proto.Limit) error
}

type TokenStore interface {
	ListByProjectID(ctx context.Context, projectID uint64, active *bool) ([]*proto.AccessToken, error)
	FindByTokenKey(ctx context.Context, tokenKey string) (*proto.AccessToken, error)
	InsertToken(ctx context.Context, token *proto.AccessToken) error
	UpdateToken(ctx context.Context, token *proto.AccessToken) (*proto.AccessToken, error)
}

type UsageStore interface {
	GetAccountTotalUsage(ctx context.Context, projectID uint64, service *proto.Service, min, max time.Time) (proto.AccessTokenUsage, error)
	UpdateTokenUsage(ctx context.Context, tokenKey string, service proto.Service, time time.Time, usage proto.AccessTokenUsage) error
}

func NewQuotaControl(cache Cache, limit LimitStore, token TokenStore, usage UsageStore) proto.QuotaControl {
	return &quotaControl{
		cache:      cache,
		limitStore: limit,
		tokenStore: token,
		usageStore: usage,
		tokenGen:   DefaultTokenKey,
	}
}

type quotaControl struct {
	cache      Cache
	limitStore LimitStore
	tokenStore TokenStore
	usageStore UsageStore
	tokenGen   func(projectID uint64) string
}

func (q quotaControl) GetUsage(ctx context.Context, projectID uint64, service *proto.Service, now time.Time) (*proto.AccessTokenUsage, error) {
	min := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())
	max := min.AddDate(0, 1, -1)
	usage, err := q.usageStore.GetAccountTotalUsage(ctx, projectID, service, min, max)
	if err != nil {
		return nil, err
	}
	return &usage, nil
}

func (q quotaControl) PrepareUsage(ctx context.Context, projectID uint64, now time.Time) (bool, error) {
	usage, err := q.GetUsage(ctx, projectID, nil, now)
	if err != nil {
		return false, err
	}
	if err := q.cache.SetComputeUnits(ctx, getQuotaKey(projectID, now), usage.GetTotalUsage()); err != nil {
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
		Limit:       limit,
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

func (q quotaControl) GetAccessLimit(ctx context.Context, projectID uint64) (*proto.Limit, error) {
	return q.limitStore.GetAccessLimit(ctx, projectID)
}

func (q quotaControl) SetAccessLimit(ctx context.Context, projectID uint64, config *proto.Limit) (bool, error) {
	if err := config.Validate(); err != nil {
		return false, proto.WebRPCError{HTTPStatus: http.StatusBadRequest, Message: err.Error()}
	}
	err := q.limitStore.SetAccessLimit(ctx, projectID, config)
	if err != nil {
		return false, err
	}
	return true, nil
}

func (q quotaControl) GetAccessToken(ctx context.Context, tokenKey string) (*proto.AccessToken, error) {
	return q.tokenStore.FindByTokenKey(ctx, tokenKey)
}

func (q quotaControl) CreateAccessToken(ctx context.Context, projectID uint64, displayName string, allowedOrigins []string, allowedServices []*proto.Service) (*proto.AccessToken, error) {
	token := proto.AccessToken{
		ProjectID:       projectID,
		DisplayName:     displayName,
		TokenKey:        q.tokenGen(projectID),
		Active:          true,
		AllowedOrigins:  allowedOrigins,
		AllowedServices: allowedServices,
	}
	if err := q.tokenStore.InsertToken(ctx, &token); err != nil {
		return nil, err
	}
	return &token, nil
}

func (q quotaControl) UpdateAccessToken(ctx context.Context, tokenKey string, displayName *string, allowedOrigins []string, allowedServices []*proto.Service) (*proto.AccessToken, error) {
	token, err := q.tokenStore.FindByTokenKey(ctx, tokenKey)
	if err != nil {
		return nil, err
	}
	if displayName != nil {
		token.DisplayName = *displayName
	}
	if allowedOrigins != nil {
		token.AllowedOrigins = allowedOrigins
	}
	if allowedServices != nil {
		token.AllowedServices = allowedServices
	}
	if token, err = q.tokenStore.UpdateToken(ctx, token); err != nil {
		return nil, err
	}
	return token, nil
}

func (q quotaControl) ListAccessTokens(ctx context.Context, projectID uint64) ([]*proto.AccessToken, error) {
	return q.tokenStore.ListByProjectID(ctx, projectID, nil)
}

func (q quotaControl) DisableAccessToken(ctx context.Context, tokenKey string) (bool, error) {
	token, err := q.tokenStore.FindByTokenKey(ctx, tokenKey)
	if err != nil {
		return false, err
	}
	token.Active = false
	if _, err := q.tokenStore.UpdateToken(ctx, token); err != nil {
		return false, err
	}
	return true, nil
}

func DefaultTokenKey(projectID uint64) string {
	buf := make([]byte, 24)
	binary.BigEndian.PutUint64(buf, projectID)
	rand.Read(buf[8:])
	return base62.EncodeToString(buf)
}

func GetProjectID(tokenKey string) (uint64, error) {
	buf, err := base62.DecodeString(tokenKey)
	if err != nil || len(buf) < 8 {
		return 0, proto.ErrTokenNotFound
	}
	return binary.BigEndian.Uint64(buf[:8]), nil
}
