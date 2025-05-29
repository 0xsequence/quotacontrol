package quotacontrol

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/0xsequence/quotacontrol/encoding"
	"github.com/0xsequence/quotacontrol/internal/store"
	"github.com/0xsequence/quotacontrol/middleware"
	"github.com/0xsequence/quotacontrol/proto"
	"github.com/go-chi/httprate"
	"github.com/goware/logadapter-slog"
	"github.com/goware/validation"
)

type LimitStore interface {
	GetAccessLimit(ctx context.Context, projectID uint64, cycle *proto.Cycle) (*proto.Limit, error)
}

type AccessKeyStore interface {
	ListAccessKeys(ctx context.Context, projectID uint64, active *bool, service *proto.Service) ([]*proto.AccessKey, error)
	FindAccessKey(ctx context.Context, accessKey string) (*proto.AccessKey, error)
	InsertAccessKey(ctx context.Context, accessKey *proto.AccessKey) error
	UpdateAccessKey(ctx context.Context, accessKey *proto.AccessKey) (*proto.AccessKey, error)
}

type UsageStore interface {
	GetAccessKeyUsage(ctx context.Context, projectID uint64, accessKey string, service *proto.Service, min, max time.Time) (proto.AccessUsage, error)
	GetAccountUsage(ctx context.Context, projectID uint64, service *proto.Service, min, max time.Time) (proto.AccessUsage, error)
	UpdateAccessUsage(ctx context.Context, projectID uint64, accessKey string, service proto.Service, time time.Time, usage proto.AccessUsage) error
}

type CycleStore interface {
	GetAccessCycle(ctx context.Context, projectID uint64, now time.Time) (*proto.Cycle, error)
}

// PermissionStore is the interface that wraps the GetUserPermission method.
type PermissionStore interface {
	GetUserPermission(ctx context.Context, projectID uint64, userID string) (proto.UserPermission, *proto.ResourceAccess, error)
}

type Cache struct {
	QuotaCache
	UsageCache
	PermissionCache
}

type Store struct {
	LimitStore
	AccessKeyStore
	UsageStore
	CycleStore
	PermissionStore
}

// NewHandler returns server implementation for proto.QuotaControl.
func NewHandler(redis RedisConfig, log *slog.Logger, cache Cache, storage Store) proto.QuotaControl {
	if log == nil {
		log = slog.Default()
	}
	if storage.CycleStore == nil {
		storage.CycleStore = store.Cycle{}
	}

	return &handler{
		log:        log.With("service", "quotacontrol"),
		cache:      cache,
		store:      storage,
		keyVersion: proto.DefaultEncoding.Version(),
		redis:      redis,
	}
}

// handler is the quotacontrol handler backend implementation.
type handler struct {
	log        *slog.Logger
	cache      Cache
	store      Store
	keyVersion byte
	redis      RedisConfig
}

var _ proto.QuotaControl = &handler{}

func (h handler) GetTimeRange(ctx context.Context, projectID uint64, from, to *time.Time) (time.Time, time.Time, error) {
	if from != nil && to != nil {
		return *from, *to, nil
	}
	now := middleware.GetTime(ctx)
	cycle, err := h.store.CycleStore.GetAccessCycle(ctx, projectID, now)
	if err != nil {
		return time.Time{}, time.Time{}, err
	}
	if from == nil && to == nil {
		cycle, _ := store.Cycle{}.GetAccessCycle(ctx, projectID, now)
		return cycle.GetStart(now), cycle.GetEnd(now), nil
	}
	duration := cycle.GetDuration(now)
	if from == nil {
		return to.Add(-duration), *to, nil
	}
	return *from, from.Add(duration), nil
}

func (h handler) GetAccountUsage(ctx context.Context, projectID uint64, service *proto.Service, from, to *time.Time) (*proto.AccessUsage, error) {
	min, max, err := h.GetTimeRange(ctx, projectID, from, to)
	if err != nil {
		return nil, err
	}

	usage, err := h.store.UsageStore.GetAccountUsage(ctx, projectID, service, min, max)
	if err != nil {
		return nil, err
	}
	return &usage, nil
}

func (h handler) GetAsyncUsage(ctx context.Context, projectID uint64, service *proto.Service, from, to *time.Time) (*proto.AccessUsage, error) {
	min, max, err := h.GetTimeRange(ctx, projectID, from, to)
	if err != nil {
		return nil, err
	}

	usage, err := h.store.UsageStore.GetAccessKeyUsage(ctx, projectID, "", service, min, max)
	if err != nil {
		return nil, err
	}
	return &usage, nil
}

func (h handler) GetAccessKeyUsage(ctx context.Context, accessKey string, service *proto.Service, from, to *time.Time) (*proto.AccessUsage, error) {
	projectID, err := proto.GetProjectID(accessKey)
	if err != nil {
		return nil, err
	}

	min, max, err := h.GetTimeRange(ctx, projectID, from, to)
	if err != nil {
		return nil, err
	}

	usage, err := h.store.UsageStore.GetAccessKeyUsage(ctx, projectID, accessKey, service, min, max)
	if err != nil {
		return nil, err
	}
	return &usage, nil
}

func (h handler) PrepareUsage(ctx context.Context, projectID uint64, cycle *proto.Cycle, now time.Time) (bool, error) {
	min, max := cycle.GetStart(now), cycle.GetEnd(now)
	usage, err := h.GetAccountUsage(ctx, projectID, nil, &min, &max)
	if err != nil {
		return false, err
	}

	key := cacheKeyQuota(projectID, cycle, now)
	if err := h.cache.UsageCache.SetUsage(ctx, key, usage.GetTotalUsage()); err != nil {
		return false, err
	}
	return true, nil
}

func (h handler) ClearUsage(ctx context.Context, projectID uint64, now time.Time) (bool, error) {
	cycle, err := h.store.CycleStore.GetAccessCycle(ctx, projectID, now)
	if err != nil {
		return false, err
	}

	key := cacheKeyQuota(projectID, cycle, now)
	ok, err := h.cache.UsageCache.ClearUsage(ctx, key)
	if err != nil {
		return false, err
	}
	return ok, nil
}

func (h handler) GetProjectQuota(ctx context.Context, projectID uint64, now time.Time) (*proto.AccessQuota, error) {
	cycle, err := h.store.CycleStore.GetAccessCycle(ctx, projectID, now)
	if err != nil {
		return nil, err
	}

	limit, err := h.store.LimitStore.GetAccessLimit(ctx, projectID, cycle)
	if err != nil {
		return nil, err
	}

	record := proto.AccessQuota{
		Limit:     limit,
		Cycle:     cycle,
		AccessKey: &proto.AccessKey{ProjectID: projectID},
	}

	if err := h.cache.QuotaCache.SetProjectQuota(ctx, &record); err != nil {
		h.log.Error("set access quota in cache", slog.Any("error", err))
	}

	return &record, nil
}

func (h handler) GetAccessQuota(ctx context.Context, accessKey string, now time.Time) (*proto.AccessQuota, error) {
	access, err := h.store.AccessKeyStore.FindAccessKey(ctx, accessKey)
	if err != nil {
		return nil, err
	}
	cycle, err := h.store.CycleStore.GetAccessCycle(ctx, access.ProjectID, now)
	if err != nil {
		return nil, err
	}
	limit, err := h.store.LimitStore.GetAccessLimit(ctx, access.ProjectID, cycle)
	if err != nil {
		return nil, err
	}
	record := proto.AccessQuota{
		Limit:     limit,
		Cycle:     cycle,
		AccessKey: access,
	}

	if err := h.cache.QuotaCache.SetAccessQuota(ctx, &record); err != nil {
		h.log.Error("set access quota in cache", slog.Any("error", err))
	}

	return &record, nil
}

func (h handler) NotifyEvent(ctx context.Context, projectID uint64, eventType proto.EventType) (bool, error) {
	h.log.Info("notify event", slog.Uint64("projectID", projectID), slog.String("eventType", eventType.String()))
	return true, nil
}

func (h handler) UpdateProjectUsage(ctx context.Context, service proto.Service, now time.Time, usage map[uint64]*proto.AccessUsage) (map[uint64]bool, error) {
	var errs []error
	m := make(map[uint64]bool, len(usage))
	for projectID, accessUsage := range usage {
		err := h.store.UsageStore.UpdateAccessUsage(ctx, projectID, "", service, now, *accessUsage)
		if err != nil {
			errs = append(errs, fmt.Errorf("%d: %w", projectID, err))
		}
		m[projectID] = err == nil
	}
	if len(errs) > 0 {
		return m, errors.Join(errs...)
	}
	return m, nil
}

func (h handler) UpdateKeyUsage(ctx context.Context, service proto.Service, now time.Time, usage map[string]*proto.AccessUsage) (map[string]bool, error) {
	var errs []error
	m := make(map[string]bool, len(usage))
	for key, u := range usage {
		projectID, err := proto.GetProjectID(key)
		if err != nil {
			errs = append(errs, fmt.Errorf("%s: %w", key, err))
			continue
		}
		if err = h.store.UsageStore.UpdateAccessUsage(ctx, projectID, key, service, now, *u); err != nil {
			errs = append(errs, fmt.Errorf("%s: %w", key, err))
		}
		m[key] = err == nil
	}
	if len(errs) > 0 {
		return m, errors.Join(errs...)
	}
	return m, nil
}

func (h handler) UpdateUsage(ctx context.Context, service proto.Service, now time.Time, usage map[string]*proto.AccessUsage) (map[string]bool, error) {
	return h.UpdateKeyUsage(ctx, service, now, usage)
}

func (h handler) ClearAccessQuotaCache(ctx context.Context, projectID uint64) (bool, error) {
	accessKeys, err := h.ListAccessKeys(ctx, projectID, proto.Ptr(true), nil)
	if err != nil {
		h.log.Error("list access keys", slog.Any("error", err))
		return true, nil
	}
	if err := h.cache.QuotaCache.DeleteProjectQuota(ctx, projectID); err != nil {
		h.log.Error("delete access quota from cache", slog.Any("error", err))
	}
	for _, access := range accessKeys {
		if err := h.cache.QuotaCache.DeleteAccessQuota(ctx, access.AccessKey); err != nil {
			h.log.Error("delete access quota from cache", slog.Any("error", err))
		}
	}
	return true, nil
}

func (h handler) GetAccessKey(ctx context.Context, accessKey string) (*proto.AccessKey, error) {
	return h.store.AccessKeyStore.FindAccessKey(ctx, accessKey)
}

func (h handler) GetDefaultAccessKey(ctx context.Context, projectID uint64) (*proto.AccessKey, error) {
	list, err := h.store.AccessKeyStore.ListAccessKeys(ctx, projectID, proto.Ptr(true), nil)
	if err != nil {
		return nil, err
	}

	for _, accessKey := range list {
		if accessKey.Default {
			return accessKey, nil
		}
	}
	return nil, proto.ErrNoDefaultKey
}

func (h handler) CreateAccessKey(ctx context.Context, projectID uint64, displayName string, requireOrigin bool, allowedOrigins []string, allowedServices []proto.Service) (*proto.AccessKey, error) {
	cycle, err := h.store.CycleStore.GetAccessCycle(ctx, projectID, middleware.GetTime(ctx))
	if err != nil {
		return nil, err
	}
	limit, err := h.store.LimitStore.GetAccessLimit(ctx, projectID, cycle)
	if err != nil {
		return nil, err
	}

	list, err := h.store.AccessKeyStore.ListAccessKeys(ctx, projectID, proto.Ptr(true), nil)
	if err != nil {
		return nil, err
	}

	if limit.MaxKeys > 0 {
		if l := len(list); int64(l) >= limit.MaxKeys {
			return nil, proto.ErrMaxAccessKeys
		}
	}

	origins, err := validation.NewOrigins(allowedOrigins...)
	if err != nil {
		return nil, err
	}

	// set key version if not set
	if _, ok := encoding.GetVersion(ctx); !ok {
		ctx = encoding.WithVersion(ctx, h.keyVersion)
	}

	k := proto.AccessKey{
		ProjectID:       projectID,
		DisplayName:     displayName,
		AccessKey:       proto.GenerateAccessKey(ctx, projectID),
		Active:          true,
		Default:         len(list) == 0,
		RequireOrigin:   requireOrigin,
		AllowedOrigins:  origins,
		AllowedServices: allowedServices,
	}
	if err := h.store.AccessKeyStore.InsertAccessKey(ctx, &k); err != nil {
		return nil, err
	}
	return &k, nil
}

func (h handler) RotateAccessKey(ctx context.Context, accessKey string) (*proto.AccessKey, error) {
	k, err := h.store.AccessKeyStore.FindAccessKey(ctx, accessKey)
	if err != nil {
		return nil, err
	}

	isDefaultKey := k.Default

	k.Active = false
	k.Default = false

	if _, err := h.updateAccessKey(ctx, k); err != nil {
		return nil, err
	}

	newAccess, err := h.CreateAccessKey(ctx, k.ProjectID, k.DisplayName, k.RequireOrigin, k.AllowedOrigins.ToStrings(), k.AllowedServices)
	if err != nil {
		return nil, err
	}

	// set new key as default
	if isDefaultKey {
		newAccess.Default = true
		return h.updateAccessKey(ctx, newAccess)
	}

	return newAccess, nil
}

func (h handler) UpdateAccessKey(ctx context.Context, accessKey string, displayName *string, requireOrigin *bool, allowedOrigins []string, allowedServices []proto.Service) (*proto.AccessKey, error) {
	k, err := h.store.AccessKeyStore.FindAccessKey(ctx, accessKey)
	if err != nil {
		return nil, err
	}

	if displayName != nil {
		k.DisplayName = *displayName
	}
	if requireOrigin != nil {
		k.RequireOrigin = *requireOrigin
	}
	if allowedOrigins != nil {
		origins, err := validation.NewOrigins(allowedOrigins...)
		if err != nil {
			return nil, err
		}
		k.AllowedOrigins = origins
	}
	if allowedServices != nil {
		k.AllowedServices = allowedServices
	}

	if k, err = h.updateAccessKey(ctx, k); err != nil {
		return nil, err
	}
	return k, nil
}

func (h handler) UpdateDefaultAccessKey(ctx context.Context, projectID uint64, accessKey string) (bool, error) {
	// make sure accessKey exists
	k, err := h.store.AccessKeyStore.FindAccessKey(ctx, accessKey)
	if err != nil {
		return false, err
	}

	if k.ProjectID != projectID {
		return false, fmt.Errorf("project doesn't own the given access key")
	}

	defaultKey, err := h.GetDefaultAccessKey(ctx, projectID)
	if err != nil {
		return false, err
	}

	// make sure new default access key & old default access key are different
	if defaultKey.AccessKey == k.AccessKey {
		return true, nil
	}

	// update old default access
	defaultKey.Default = false
	if _, err := h.updateAccessKey(ctx, defaultKey); err != nil {
		return false, err
	}

	// set new access key to default
	k.Default = true
	if _, err = h.updateAccessKey(ctx, k); err != nil {
		return false, err
	}

	return true, nil
}

func (h handler) ListAccessKeys(ctx context.Context, projectID uint64, active *bool, service *proto.Service) ([]*proto.AccessKey, error) {
	return h.store.AccessKeyStore.ListAccessKeys(ctx, projectID, active, service)
}

func (h handler) DisableAccessKey(ctx context.Context, accessKey string) (bool, error) {
	k, err := h.store.AccessKeyStore.FindAccessKey(ctx, accessKey)
	if err != nil {
		return false, err
	}

	list, err := h.store.AccessKeyStore.ListAccessKeys(ctx, k.ProjectID, proto.Ptr(true), nil)
	if err != nil {
		return false, err
	}

	if len(list) == 1 {
		return false, proto.ErrAtLeastOneKey
	}

	k.Active = false
	k.Default = false
	if _, err := h.updateAccessKey(ctx, k); err != nil {
		return false, err
	}

	// set another project accessKey to default
	if _, err := h.GetDefaultAccessKey(ctx, k.ProjectID); err == proto.ErrNoDefaultKey {
		listUpdated, err := h.store.AccessKeyStore.ListAccessKeys(ctx, k.ProjectID, proto.Ptr(true), nil)
		if err != nil {
			return false, err
		}

		newDefaultKey := listUpdated[0]
		newDefaultKey.Default = true

		if _, err = h.updateAccessKey(ctx, newDefaultKey); err != nil {
			return false, err
		}
	}

	return true, nil
}

func (h handler) GetUserPermission(ctx context.Context, projectID uint64, userID string) (proto.UserPermission, *proto.ResourceAccess, error) {
	perm, access, err := h.store.PermissionStore.GetUserPermission(ctx, projectID, userID)
	if err != nil {
		return proto.UserPermission_UNAUTHORIZED, nil, proto.ErrUnauthorizedUser
	}

	if !perm.Is(proto.UserPermission_UNAUTHORIZED) {
		if err := h.cache.PermissionCache.SetUserPermission(ctx, projectID, userID, perm, access); err != nil {
			h.log.Error("set user perm in cache", slog.Any("error", err))
		}
	}

	return perm, access, nil
}

func (h handler) updateAccessKey(ctx context.Context, k *proto.AccessKey) (*proto.AccessKey, error) {
	k, err := h.store.AccessKeyStore.UpdateAccessKey(ctx, k)
	if err != nil {
		return nil, err
	}

	if err := h.cache.QuotaCache.DeleteAccessQuota(ctx, k.AccessKey); err != nil {
		h.log.Error("delete access quota from cache", slog.Any("error", err))
	}

	return k, nil
}

func (h handler) GetProjectStatus(ctx context.Context, projectID uint64) (*proto.ProjectStatus, error) {
	status := proto.ProjectStatus{
		ProjectID: projectID,
	}

	now := middleware.GetTime(ctx)
	cycle, err := h.store.CycleStore.GetAccessCycle(ctx, projectID, now)
	if err != nil {
		return nil, err
	}

	limit, err := h.store.LimitStore.GetAccessLimit(ctx, projectID, cycle)
	if err != nil {
		return nil, err
	}
	status.Limit = limit

	cacheKey := cacheKeyQuota(projectID, cycle, now)
	usage, err := h.cache.UsageCache.PeekUsage(ctx, cacheKey)
	if err != nil {
		if !errors.Is(err, ErrCachePing) {
			return nil, err
		}
		if _, err := h.PrepareUsage(ctx, projectID, cycle, now); err != nil {
			return nil, err
		}
		if usage, err = h.cache.UsageCache.PeekUsage(ctx, cacheKey); err != nil {
			return nil, err
		}
	}
	status.UsageCounter = usage

	wlog := logadapter.LogAdapter(h.log)
	status.RateLimitCounter = make(map[proto.Service]int64)

	for i := range proto.Service_name {
		service := proto.Service(i)
		limitCounter := NewLimitCounter(service, h.redis, wlog)

		limiter := httprate.NewRateLimiter(int(limit.RateLimit), time.Minute, httprate.WithLimitCounter(limitCounter))
		_, rate, err := limiter.Status(middleware.ProjectRateKey(projectID) + ":")
		if err != nil {
			return nil, err
		}
		status.RateLimitCounter[service] = int64(rate)

	}

	return &status, nil
}
