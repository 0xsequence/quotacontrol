package quotacontrol

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/0xsequence/authcontrol"
	"github.com/0xsequence/quotacontrol/internal/store"
	"github.com/0xsequence/quotacontrol/middleware"
	"github.com/0xsequence/quotacontrol/proto"
	"github.com/go-chi/httprate"
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

// NewServer returns server implementation for proto.QuotaControl.
func NewServer(redis RedisConfig, log *slog.Logger, cache Cache, storage Store) proto.QuotaControl {
	if log == nil {
		log = slog.Default()
	}
	if storage.CycleStore == nil {
		storage.CycleStore = store.Cycle{}
	}

	return &server{
		log:        log.With("service", "quotacontrol"),
		cache:      cache,
		store:      storage,
		keyVersion: authcontrol.DefaultEncoding.Version(),
		redis:      redis,
	}
}

// server is the quotacontrol server backend implementation.
type server struct {
	log        *slog.Logger
	cache      Cache
	store      Store
	keyVersion byte
	redis      RedisConfig
}

var _ proto.QuotaControl = &server{}

func (s server) GetTimeRange(ctx context.Context, projectID uint64, from, to *time.Time) (time.Time, time.Time, error) {
	if from != nil && to != nil {
		return *from, *to, nil
	}
	now := middleware.GetTime(ctx)
	cycle, err := s.store.CycleStore.GetAccessCycle(ctx, projectID, now)
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

func (s server) GetAccountUsage(ctx context.Context, projectID uint64, service *proto.Service, from, to *time.Time) (*proto.AccessUsage, error) {
	min, max, err := s.GetTimeRange(ctx, projectID, from, to)
	if err != nil {
		return nil, err
	}

	usage, err := s.store.UsageStore.GetAccountUsage(ctx, projectID, service, min, max)
	if err != nil {
		return nil, err
	}
	return &usage, nil
}

func (s server) GetAsyncUsage(ctx context.Context, projectID uint64, service *proto.Service, from, to *time.Time) (*proto.AccessUsage, error) {
	min, max, err := s.GetTimeRange(ctx, projectID, from, to)
	if err != nil {
		return nil, err
	}

	usage, err := s.store.UsageStore.GetAccessKeyUsage(ctx, projectID, "", service, min, max)
	if err != nil {
		return nil, err
	}
	return &usage, nil
}

func (s server) GetAccessKeyUsage(ctx context.Context, accessKey string, service *proto.Service, from, to *time.Time) (*proto.AccessUsage, error) {
	projectID, err := authcontrol.GetProjectIDFromAccessKey(accessKey)
	if err != nil {
		return nil, err
	}

	min, max, err := s.GetTimeRange(ctx, projectID, from, to)
	if err != nil {
		return nil, err
	}

	usage, err := s.store.UsageStore.GetAccessKeyUsage(ctx, projectID, accessKey, service, min, max)
	if err != nil {
		return nil, err
	}
	return &usage, nil
}

func (s server) PrepareUsage(ctx context.Context, projectID uint64, service *proto.Service, cycle *proto.Cycle, now time.Time) (bool, error) {
	min, max := cycle.GetStart(now), cycle.GetEnd(now)
	usage, err := s.GetAccountUsage(ctx, projectID, nil, &min, &max)
	if err != nil {
		return false, err
	}

	key := cacheKeyQuota(projectID, cycle, service, now)
	if err := s.cache.UsageCache.SetUsage(ctx, key, usage.GetTotalUsage()); err != nil {
		return false, err
	}
	return true, nil
}

func (s server) ClearUsage(ctx context.Context, projectID uint64, service *proto.Service, now time.Time) (bool, error) {
	cycle, err := s.store.CycleStore.GetAccessCycle(ctx, projectID, now)
	if err != nil {
		return false, err
	}

	if service != nil {
		key := cacheKeyQuota(projectID, cycle, service, now)
		ok, err := s.cache.UsageCache.ClearUsage(ctx, key)
		if err != nil {
			return false, err
		}
		return ok, nil
	}

	for i := range proto.Service_name {
		svc := proto.Service(i)
		key := cacheKeyQuota(projectID, cycle, &svc, now)
		if _, err := s.cache.UsageCache.ClearUsage(ctx, key); err != nil {
			return false, err
		}
	}
	return true, nil
}

func (s server) GetProjectQuota(ctx context.Context, projectID uint64, now time.Time) (*proto.AccessQuota, error) {
	cycle, err := s.store.CycleStore.GetAccessCycle(ctx, projectID, now)
	if err != nil {
		return nil, err
	}

	limit, err := s.store.LimitStore.GetAccessLimit(ctx, projectID, cycle)
	if err != nil {
		return nil, err
	}
	limit.PopulateLegacyFields()

	record := proto.AccessQuota{
		Limit:     limit,
		Cycle:     cycle,
		AccessKey: &proto.AccessKey{ProjectID: projectID},
	}

	if err := s.cache.QuotaCache.SetProjectQuota(ctx, &record); err != nil {
		s.log.Error("set access quota in cache", slog.Any("error", err))
	}

	return &record, nil
}

func (s server) GetAccessQuota(ctx context.Context, accessKey string, now time.Time) (*proto.AccessQuota, error) {
	access, err := s.store.AccessKeyStore.FindAccessKey(ctx, accessKey)
	if err != nil {
		return nil, err
	}
	cycle, err := s.store.CycleStore.GetAccessCycle(ctx, access.ProjectID, now)
	if err != nil {
		return nil, err
	}
	limit, err := s.store.LimitStore.GetAccessLimit(ctx, access.ProjectID, cycle)
	if err != nil {
		return nil, err
	}
	limit.PopulateLegacyFields()

	record := proto.AccessQuota{
		Limit:     limit,
		Cycle:     cycle,
		AccessKey: access,
	}

	if err := s.cache.QuotaCache.SetAccessQuota(ctx, &record); err != nil {
		s.log.Error("set access quota in cache", slog.Any("error", err))
	}

	return &record, nil
}

func (s server) NotifyEvent(ctx context.Context, projectID uint64, eventType proto.EventType) (bool, error) {
	s.log.Info("notify event", slog.Uint64("projectID", projectID), slog.String("eventType", eventType.String()))
	return true, nil
}

func (s server) UpdateProjectUsage(ctx context.Context, service proto.Service, now time.Time, usage map[uint64]*proto.AccessUsage) (map[uint64]bool, error) {
	var errs []error
	m := make(map[uint64]bool, len(usage))
	for projectID, accessUsage := range usage {
		err := s.store.UsageStore.UpdateAccessUsage(ctx, projectID, "", service, now, *accessUsage)
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

func (s server) UpdateKeyUsage(ctx context.Context, service proto.Service, now time.Time, usage map[string]*proto.AccessUsage) (map[string]bool, error) {
	var errs []error
	m := make(map[string]bool, len(usage))
	for key, u := range usage {
		projectID, err := authcontrol.GetProjectIDFromAccessKey(key)
		if err != nil {
			errs = append(errs, fmt.Errorf("%s: %w", key, err))
			continue
		}
		if err = s.store.UsageStore.UpdateAccessUsage(ctx, projectID, key, service, now, *u); err != nil {
			errs = append(errs, fmt.Errorf("%s: %w", key, err))
		}
		m[key] = err == nil
	}
	if len(errs) > 0 {
		return m, errors.Join(errs...)
	}
	return m, nil
}

func (s server) UpdateUsage(ctx context.Context, service proto.Service, now time.Time, usage map[string]*proto.AccessUsage) (map[string]bool, error) {
	return s.UpdateKeyUsage(ctx, service, now, usage)
}

func (s server) ClearAccessQuotaCache(ctx context.Context, projectID uint64) (bool, error) {
	accessKeys, err := s.ListAccessKeys(ctx, projectID, proto.Ptr(true), nil)
	if err != nil {
		s.log.Error("list access keys", slog.Any("error", err))
		return true, nil
	}
	if err := s.cache.QuotaCache.DeleteProjectQuota(ctx, projectID); err != nil {
		s.log.Error("delete access quota from cache", slog.Any("error", err))
	}
	for _, access := range accessKeys {
		if err := s.cache.QuotaCache.DeleteAccessQuota(ctx, access.AccessKey); err != nil {
			s.log.Error("delete access quota from cache", slog.Any("error", err))
		}
	}
	return true, nil
}

func (s server) GetAccessKey(ctx context.Context, accessKey string) (*proto.AccessKey, error) {
	return s.store.AccessKeyStore.FindAccessKey(ctx, accessKey)
}

func (s server) GetDefaultAccessKey(ctx context.Context, projectID uint64) (*proto.AccessKey, error) {
	list, err := s.store.AccessKeyStore.ListAccessKeys(ctx, projectID, proto.Ptr(true), nil)
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

func (s server) CreateAccessKey(ctx context.Context, projectID uint64, displayName string, requireOrigin bool, allowedOrigins []string, allowedServices []proto.Service) (*proto.AccessKey, error) {
	cycle, err := s.store.CycleStore.GetAccessCycle(ctx, projectID, middleware.GetTime(ctx))
	if err != nil {
		return nil, err
	}
	limit, err := s.store.LimitStore.GetAccessLimit(ctx, projectID, cycle)
	if err != nil {
		return nil, err
	}

	list, err := s.store.AccessKeyStore.ListAccessKeys(ctx, projectID, proto.Ptr(true), nil)
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
	if _, ok := authcontrol.GetVersion(ctx); !ok {
		ctx = authcontrol.WithVersion(ctx, s.keyVersion)
	}

	k := proto.AccessKey{
		ProjectID:       projectID,
		DisplayName:     displayName,
		AccessKey:       authcontrol.GenerateAccessKey(ctx, projectID),
		Active:          true,
		Default:         len(list) == 0,
		RequireOrigin:   requireOrigin,
		AllowedOrigins:  origins,
		AllowedServices: allowedServices,
	}
	if err := s.store.AccessKeyStore.InsertAccessKey(ctx, &k); err != nil {
		return nil, err
	}
	return &k, nil
}

func (s server) RotateAccessKey(ctx context.Context, accessKey string) (*proto.AccessKey, error) {
	k, err := s.store.AccessKeyStore.FindAccessKey(ctx, accessKey)
	if err != nil {
		return nil, err
	}

	isDefaultKey := k.Default

	k.Active = false
	k.Default = false

	if _, err := s.updateAccessKey(ctx, k); err != nil {
		return nil, err
	}

	newAccess, err := s.CreateAccessKey(ctx, k.ProjectID, k.DisplayName, k.RequireOrigin, k.AllowedOrigins.ToStrings(), k.AllowedServices)
	if err != nil {
		return nil, err
	}

	// set new key as default
	if isDefaultKey {
		newAccess.Default = true
		return s.updateAccessKey(ctx, newAccess)
	}

	return newAccess, nil
}

func (s server) UpdateAccessKey(ctx context.Context, accessKey string, displayName *string, requireOrigin *bool, allowedOrigins []string, allowedServices []proto.Service) (*proto.AccessKey, error) {
	k, err := s.store.AccessKeyStore.FindAccessKey(ctx, accessKey)
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

	if k, err = s.updateAccessKey(ctx, k); err != nil {
		return nil, err
	}
	return k, nil
}

func (s server) UpdateDefaultAccessKey(ctx context.Context, projectID uint64, accessKey string) (bool, error) {
	// make sure accessKey exists
	k, err := s.store.AccessKeyStore.FindAccessKey(ctx, accessKey)
	if err != nil {
		return false, err
	}

	if k.ProjectID != projectID {
		return false, fmt.Errorf("project doesn't own the given access key")
	}

	defaultKey, err := s.GetDefaultAccessKey(ctx, projectID)
	if err != nil {
		return false, err
	}

	// make sure new default access key & old default access key are different
	if defaultKey.AccessKey == k.AccessKey {
		return true, nil
	}

	// update old default access
	defaultKey.Default = false
	if _, err := s.updateAccessKey(ctx, defaultKey); err != nil {
		return false, err
	}

	// set new access key to default
	k.Default = true
	if _, err = s.updateAccessKey(ctx, k); err != nil {
		return false, err
	}

	return true, nil
}

func (s server) ListAccessKeys(ctx context.Context, projectID uint64, active *bool, service *proto.Service) ([]*proto.AccessKey, error) {
	return s.store.AccessKeyStore.ListAccessKeys(ctx, projectID, active, service)
}

func (s server) DisableAccessKey(ctx context.Context, accessKey string) (bool, error) {
	k, err := s.store.AccessKeyStore.FindAccessKey(ctx, accessKey)
	if err != nil {
		return false, err
	}

	list, err := s.store.AccessKeyStore.ListAccessKeys(ctx, k.ProjectID, proto.Ptr(true), nil)
	if err != nil {
		return false, err
	}

	if len(list) == 1 {
		return false, proto.ErrAtLeastOneKey
	}

	k.Active = false
	k.Default = false
	if _, err := s.updateAccessKey(ctx, k); err != nil {
		return false, err
	}

	// set another project accessKey to default
	if _, err := s.GetDefaultAccessKey(ctx, k.ProjectID); err == proto.ErrNoDefaultKey {
		listUpdated, err := s.store.AccessKeyStore.ListAccessKeys(ctx, k.ProjectID, proto.Ptr(true), nil)
		if err != nil {
			return false, err
		}

		newDefaultKey := listUpdated[0]
		newDefaultKey.Default = true

		if _, err = s.updateAccessKey(ctx, newDefaultKey); err != nil {
			return false, err
		}
	}

	return true, nil
}

func (s server) GetUserPermission(ctx context.Context, projectID uint64, userID string) (proto.UserPermission, *proto.ResourceAccess, error) {
	perm, access, err := s.store.PermissionStore.GetUserPermission(ctx, projectID, userID)
	if err != nil {
		return proto.UserPermission_UNAUTHORIZED, nil, proto.ErrUnauthorizedUser
	}

	if !perm.Is(proto.UserPermission_UNAUTHORIZED) {
		if err := s.cache.PermissionCache.SetUserPermission(ctx, projectID, userID, perm, access); err != nil {
			s.log.Error("set user perm in cache", slog.Any("error", err))
		}
	}

	return perm, access, nil
}

func (s server) updateAccessKey(ctx context.Context, k *proto.AccessKey) (*proto.AccessKey, error) {
	k, err := s.store.AccessKeyStore.UpdateAccessKey(ctx, k)
	if err != nil {
		return nil, err
	}

	if err := s.cache.QuotaCache.DeleteAccessQuota(ctx, k.AccessKey); err != nil {
		s.log.Error("delete access quota from cache", slog.Any("error", err))
	}

	return k, nil
}

func (s server) GetProjectStatus(ctx context.Context, projectID uint64) (*proto.ProjectStatus, error) {
	status := proto.ProjectStatus{
		ProjectID: projectID,
	}

	now := middleware.GetTime(ctx)
	cycle, err := s.store.CycleStore.GetAccessCycle(ctx, projectID, now)
	if err != nil {
		return nil, err
	}

	limit, err := s.store.LimitStore.GetAccessLimit(ctx, projectID, cycle)
	if err != nil {
		return nil, err
	}
	limit.PopulateLegacyFields()

	status.Limit = limit

	status.RateLimitCounter = make(map[string]int64)
	status.UsageCounter = make(map[string]int64)

	for i := range proto.Service_name {
		svc := proto.Service(i)
		name := svc.GetName()

		cacheKey := cacheKeyQuota(projectID, cycle, &svc, now)
		usage, err := s.cache.UsageCache.PeekUsage(ctx, cacheKey)
		if err != nil {
			if !errors.Is(err, ErrCachePing) {
				return nil, err
			}
			if _, err := s.PrepareUsage(ctx, projectID, &svc, cycle, now); err != nil {
				return nil, err
			}
			if usage, err = s.cache.UsageCache.PeekUsage(ctx, cacheKey); err != nil {
				return nil, err
			}
		}
		status.UsageCounter[name] = usage

		limitCounter := NewLimitCounter(svc, s.redis, s.log)

		limiter := httprate.NewRateLimiter(int(limit.GetServiceLimit(&svc).RateLimit), time.Minute, httprate.WithLimitCounter(limitCounter))
		_, rate, err := limiter.Status(middleware.ProjectRateKey(projectID) + ":")
		if err != nil {
			return nil, err
		}

		status.RateLimitCounter[name] = int64(rate)
	}

	return &status, nil
}
