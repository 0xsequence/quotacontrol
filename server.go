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

type ProjectInfoStore interface {
	GetProjectInfo(ctx context.Context, projectID uint64) (*proto.ProjectInfo, error)
}

type LimitStore interface {
	GetLimit(ctx context.Context, projectID uint64, service proto.Service) (*proto.Limit, error)
}

type AccessKeyStore interface {
	ListAccessKeys(ctx context.Context, projectID uint64, active *bool, service *proto.Service) ([]*proto.AccessKey, error)
	FindAccessKey(ctx context.Context, accessKey string) (*proto.AccessKey, error)
	InsertAccessKey(ctx context.Context, accessKey *proto.AccessKey) error
	UpdateAccessKey(ctx context.Context, accessKey *proto.AccessKey) (*proto.AccessKey, error)
}

type UsageStore interface {
	GetAccessKeyUsage(ctx context.Context, projectID uint64, accessKey string, service *proto.Service, min, max time.Time) (int64, error)
	GetAccountUsage(ctx context.Context, projectID uint64, service *proto.Service, min, max time.Time) (int64, error)
	InsertAccessUsage(ctx context.Context, projectID uint64, accessKey string, service proto.Service, time time.Time, usage int64) error
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
	ProjectInfoStore
	LimitStore
	AccessKeyStore
	UsageStore
	PermissionStore
}

// NewServer returns server implementation for proto.QuotaControl.
func NewServer(redis RedisConfig, log *slog.Logger, cache Cache, storage Store) proto.QuotaControlServer {
	if log == nil {
		log = slog.Default()
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

var _ proto.QuotaControlServer = &server{}

func (s server) GetTimeRange(ctx context.Context, projectID uint64, from, to *time.Time) (time.Time, time.Time, error) {
	if from != nil && to != nil {
		return *from, *to, nil
	}
	now := middleware.GetTime(ctx)
	info, err := s.store.ProjectInfoStore.GetProjectInfo(ctx, projectID)
	if err != nil {
		return time.Time{}, time.Time{}, err
	}
	if from == nil && to == nil {
		cycle, _ := store.Cycle{}.GetAccessCycle(ctx, projectID, now)
		return cycle.GetStart(now), cycle.GetEnd(now), nil
	}

	duration := info.Cycle.GetDuration(now)
	if from == nil {
		return to.Add(-duration), *to, nil
	}
	return *from, from.Add(duration), nil
}

// Deprecated: use GetUsage instead.
func (s server) GetAccountUsage(ctx context.Context, projectID uint64, service *proto.Service, from, to *time.Time) (*proto.AccessUsage, error) {
	usage, err := s.GetUsage(ctx, projectID, nil, service, from, to)
	if err != nil {
		return nil, fmt.Errorf("get account usage: %w", err)
	}
	return &proto.AccessUsage{ValidCompute: usage}, nil
}

// Deprecated: use GetUsage instead.
func (s server) GetAsyncUsage(ctx context.Context, projectID uint64, service *proto.Service, from, to *time.Time) (*proto.AccessUsage, error) {
	usage, err := s.GetUsage(ctx, projectID, proto.Ptr(""), service, from, to)
	if err != nil {
		return nil, fmt.Errorf("get usage: %w", err)
	}
	return &proto.AccessUsage{ValidCompute: usage}, nil
}

// Deprecated: use GetUsage instead.
func (s server) GetAccessKeyUsage(ctx context.Context, accessKey string, service *proto.Service, from, to *time.Time) (*proto.AccessUsage, error) {
	projectID, err := authcontrol.GetProjectIDFromAccessKey(accessKey)
	if err != nil {
		return nil, fmt.Errorf("get project id: %w", err)
	}

	usage, err := s.GetUsage(ctx, projectID, &accessKey, service, from, to)
	if err != nil {
		return nil, fmt.Errorf("get usage: %w", err)
	}
	return &proto.AccessUsage{ValidCompute: usage}, nil
}

func (s server) GetUsage(ctx context.Context, projectID uint64, accessKey *string, service *proto.Service, from *time.Time, to *time.Time) (int64, error) {
	min, max, err := s.GetTimeRange(ctx, projectID, from, to)
	if err != nil {
		return 0, fmt.Errorf("get time range: %w", err)
	}

	if accessKey == nil {
		// Total usage
		usage, err := s.store.UsageStore.GetAccountUsage(ctx, projectID, service, min, max)
		if err != nil {
			return 0, fmt.Errorf("get account usage: %w", err)
		}
		return usage, nil
	}

	if *accessKey == "" {
		// Async usage
		usage, err := s.store.UsageStore.GetAccessKeyUsage(ctx, projectID, "", service, min, max)
		if err != nil {
			return 0, fmt.Errorf("get async usage: %w", err)
		}
		return usage, nil
	}

	// Access key usage
	usage, err := s.store.UsageStore.GetAccessKeyUsage(ctx, projectID, *accessKey, service, min, max)
	if err != nil {
		return 0, fmt.Errorf("get access key usage: %w", err)
	}
	return usage, nil
}

// Deprecated: new version of client sets the usage cache directly. This is going to be removed in the future.
func (s server) PrepareUsage(ctx context.Context, projectID uint64, service *proto.Service, cycle *proto.Cycle, now time.Time) (bool, error) {
	min, max := cycle.GetStart(now), cycle.GetEnd(now)
	usage, err := s.GetUsage(ctx, projectID, nil, service, &min, &max)
	if err != nil {
		return false, fmt.Errorf("get account usage: %w", err)
	}

	key := cacheKeyQuota(projectID, cycle, service, now)
	if err := s.cache.UsageCache.SetUsage(ctx, key, usage); err != nil {
		return false, fmt.Errorf("set usage cache: %w", err)
	}
	return true, nil
}

func (s server) ClearUsage(ctx context.Context, projectID uint64, service *proto.Service, now time.Time) (bool, error) {
	info, err := s.store.ProjectInfoStore.GetProjectInfo(ctx, projectID)
	if err != nil {
		return false, fmt.Errorf("get project info 1: %w", err)
	}

	if service != nil {
		key := cacheKeyQuota(projectID, info.Cycle, service, now)
		ok, err := s.cache.UsageCache.ClearUsage(ctx, key)
		if err != nil {
			return false, fmt.Errorf("clear usage cache: %w", err)
		}
		return ok, nil
	}

	for i := range proto.Service_name {
		svc := proto.Service(i)
		key := cacheKeyQuota(projectID, info.Cycle, &svc, now)
		if _, err := s.cache.UsageCache.ClearUsage(ctx, key); err != nil {
			return false, fmt.Errorf("clear usage cache for service %s: %w", svc.String(), err)
		}
	}
	return true, nil
}

func (s server) getLegacyLimit(ctx context.Context, projectID uint64) (*proto.ProjectInfo, *proto.LegacyLimit, error) {
	info, err := s.store.ProjectInfoStore.GetProjectInfo(ctx, projectID)
	if err != nil {
		if errors.Is(err, proto.ErrProjectNotFound) {
			return nil, nil, err
		}
		return nil, nil, fmt.Errorf("get project info 2: %w", err)
	}

	limit := proto.LegacyLimit{
		ServiceLimit: map[string]proto.Limit{},
	}

	services := info.Services
	if len(services) == 0 {
		for i := range proto.Service_name {
			services = append(services, proto.Service(i))
		}
	}

	for _, svc := range services {
		svcLimit, err := s.store.LimitStore.GetLimit(ctx, projectID, svc)
		if err != nil {
			if errors.Is(err, proto.ErrInvalidService) {
				continue
			}
			return nil, nil, fmt.Errorf("get %s limit: %w", svc.GetName(), err)
		}
		limit.SetSetting(svc, *svcLimit)
	}
	return info, &limit, nil
}

func (s server) GetProjectQuota(ctx context.Context, projectID uint64, now time.Time) (*proto.AccessQuota, error) {
	info, limit, err := s.getLegacyLimit(ctx, projectID)
	if err != nil {
		return nil, fmt.Errorf("get legacy limit: %w", err)
	}

	record := proto.AccessQuota{
		Limit:     limit,
		Cycle:     info.Cycle,
		AccessKey: &proto.AccessKey{ProjectID: projectID},
	}

	return &record, nil
}

func (s server) GetAccessQuota(ctx context.Context, accessKey string, now time.Time) (*proto.AccessQuota, error) {
	access, err := s.store.AccessKeyStore.FindAccessKey(ctx, accessKey)
	if err != nil {
		if errors.Is(err, proto.ErrAccessKeyNotFound) || errors.Is(err, proto.ErrProjectNotFound) {
			return nil, err
		}
		return nil, fmt.Errorf("find access key: %w", err)
	}

	info, limit, err := s.getLegacyLimit(ctx, access.ProjectID)
	if err != nil {
		return nil, fmt.Errorf("get legacy limit: %w", err)
	}

	record := proto.AccessQuota{
		Limit:     limit,
		Cycle:     info.Cycle,
		AccessKey: access,
	}

	return &record, nil
}

func (s server) NotifyEvent(ctx context.Context, projectID uint64, service proto.Service, eventType proto.EventType) (bool, error) {
	s.log.Info("notify event", slog.Uint64("projectId", projectID), slog.String("service", service.GetName()), slog.String("eventType", eventType.String()))
	return true, nil
}

func (s server) SyncProjectUsage(ctx context.Context, service proto.Service, now time.Time, usage map[uint64]int64) (map[uint64]bool, error) {
	var errs []error
	m := make(map[uint64]bool, len(usage))
	for projectID, usage := range usage {
		err := s.store.UsageStore.InsertAccessUsage(ctx, projectID, "", service, now, usage)
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

// Deprecated: use SyncProjectUsage instead.
func (s server) UpdateProjectUsage(ctx context.Context, service proto.Service, now time.Time, usage map[uint64]*proto.AccessUsage) (map[uint64]bool, error) {
	m := make(map[uint64]int64, len(usage))
	for projectID, u := range usage {
		m[projectID] = u.ValidCompute
	}

	return s.SyncProjectUsage(ctx, service, now, m)
}

func (s server) SyncAccessKeyUsage(ctx context.Context, service proto.Service, now time.Time, usage map[string]int64) (map[string]bool, error) {
	var errs []error
	m := make(map[string]bool, len(usage))
	for key, usage := range usage {
		projectID, err := authcontrol.GetProjectIDFromAccessKey(key)
		if err != nil {
			errs = append(errs, fmt.Errorf("%s: %w", key, err))
			continue
		}
		if err = s.store.UsageStore.InsertAccessUsage(ctx, projectID, key, service, now, usage); err != nil {
			errs = append(errs, fmt.Errorf("%s: %w", key, err))
		}
		m[key] = err == nil
	}
	if len(errs) > 0 {
		return m, errors.Join(errs...)
	}
	return m, nil
}

// Deprecated: use SyncAccessKeyUsage instead.
func (s server) UpdateKeyUsage(ctx context.Context, service proto.Service, now time.Time, usage map[string]*proto.AccessUsage) (map[string]bool, error) {
	m := make(map[string]int64, len(usage))
	for accessKey, u := range usage {
		m[accessKey] = u.ValidCompute
	}
	return s.SyncAccessKeyUsage(ctx, service, now, m)
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

func (s server) GetDefaultAccessKey(ctx context.Context, projectID uint64) (*proto.AccessKey, error) {
	list, err := s.store.AccessKeyStore.ListAccessKeys(ctx, projectID, proto.Ptr(true), nil)
	if err != nil {
		return nil, fmt.Errorf("list access keys: %w", err)
	}

	for _, accessKey := range list {
		if accessKey.Default {
			return accessKey, nil
		}
	}
	return nil, proto.ErrNoDefaultKey
}

func (s server) CreateAccessKey(ctx context.Context, projectID uint64, displayName string, requireOrigin bool, allowedOrigins []string, allowedServices []proto.Service) (*proto.AccessKey, error) {
	list, err := s.store.AccessKeyStore.ListAccessKeys(ctx, projectID, proto.Ptr(true), nil)
	if err != nil {
		return nil, fmt.Errorf("list access keys: %w", err)
	}

	origins, err := validation.NewOrigins(allowedOrigins...)
	if err != nil {
		return nil, fmt.Errorf("validate allowed origins: %w", err)
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
		return nil, fmt.Errorf("insert access key: %w", err)
	}
	return &k, nil
}

func (s server) RotateAccessKey(ctx context.Context, accessKey string) (*proto.AccessKey, error) {
	existing, err := s.store.AccessKeyStore.FindAccessKey(ctx, accessKey)
	if err != nil {
		return nil, fmt.Errorf("find access key: %w", err)
	}

	isDefaultKey := existing.Default

	existing.Active = false
	existing.Default = false

	if _, err := s.updateAccessKey(ctx, existing); err != nil {
		return nil, fmt.Errorf("update access key: %w", err)
	}

	newKey, err := s.CreateAccessKey(ctx, existing.ProjectID, existing.DisplayName, existing.RequireOrigin, existing.AllowedOrigins.ToStrings(), existing.AllowedServices)
	if err != nil {
		return nil, fmt.Errorf("create access key: %w", err)
	}

	// set new key as default
	if isDefaultKey {
		newKey.Default = true
		if newKey, err = s.updateAccessKey(ctx, newKey); err != nil {
			return nil, fmt.Errorf("update access key: %w", err)
		}
	}

	return newKey, nil
}

func (s server) UpdateAccessKey(ctx context.Context, accessKey string, displayName *string, requireOrigin *bool, allowedOrigins []string, allowedServices []proto.Service) (*proto.AccessKey, error) {
	k, err := s.store.AccessKeyStore.FindAccessKey(ctx, accessKey)
	if err != nil {
		return nil, fmt.Errorf("find access key: %w", err)
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
			return nil, fmt.Errorf("validate allowed origins: %w", err)
		}
		k.AllowedOrigins = origins
	}
	if allowedServices != nil {
		k.AllowedServices = allowedServices
	}

	if k, err = s.updateAccessKey(ctx, k); err != nil {
		return nil, fmt.Errorf("update access key: %w", err)
	}
	return k, nil
}

func (s server) SetDefaultAccessKey(ctx context.Context, projectID uint64, accessKey string) (bool, error) {
	// make sure accessKey exists
	k, err := s.store.AccessKeyStore.FindAccessKey(ctx, accessKey)
	if err != nil {
		return false, fmt.Errorf("find access key: %w", err)
	}

	if k.ProjectID != projectID {
		return false, proto.ErrPermissionDenied.WithCausef("project doesn't own the given access key")
	}

	defaultKey, err := s.GetDefaultAccessKey(ctx, projectID)
	if err != nil {
		return false, fmt.Errorf("get default access key: %w", err)
	}

	// make sure new default access key & old default access key are different
	if defaultKey.AccessKey == k.AccessKey {
		return true, nil
	}

	// update old default access
	defaultKey.Default = false
	if _, err := s.updateAccessKey(ctx, defaultKey); err != nil {
		return false, fmt.Errorf("update old default access key: %w", err)
	}

	// set new access key to default
	k.Default = true
	if _, err = s.updateAccessKey(ctx, k); err != nil {
		return false, fmt.Errorf("update new default access key: %w", err)
	}

	return true, nil
}

func (s server) ListAccessKeys(ctx context.Context, projectID uint64, active *bool, service *proto.Service) ([]*proto.AccessKey, error) {
	return s.store.AccessKeyStore.ListAccessKeys(ctx, projectID, active, service)
}

func (s server) DisableAccessKey(ctx context.Context, accessKey string) (bool, error) {
	k, err := s.store.AccessKeyStore.FindAccessKey(ctx, accessKey)
	if err != nil {
		return false, fmt.Errorf("find access key: %w", err)
	}

	list, err := s.store.AccessKeyStore.ListAccessKeys(ctx, k.ProjectID, proto.Ptr(true), nil)
	if err != nil {
		return false, fmt.Errorf("list access keys: %w", err)
	}

	if len(list) == 1 {
		return false, proto.ErrAtLeastOneKey
	}

	k.Active = false
	k.Default = false
	if _, err := s.updateAccessKey(ctx, k); err != nil {
		return false, fmt.Errorf("update access key: %w", err)
	}

	// set another project accessKey to default
	if _, err := s.GetDefaultAccessKey(ctx, k.ProjectID); err == proto.ErrNoDefaultKey {
		listUpdated, err := s.store.AccessKeyStore.ListAccessKeys(ctx, k.ProjectID, proto.Ptr(true), nil)
		if err != nil {
			return false, fmt.Errorf("list access keys: %w", err)
		}

		newDefaultKey := listUpdated[0]
		newDefaultKey.Default = true

		if _, err = s.updateAccessKey(ctx, newDefaultKey); err != nil {
			return false, fmt.Errorf("update new default access key: %w", err)
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

	info, limit, err := s.getLegacyLimit(ctx, projectID)
	if err != nil {
		return nil, fmt.Errorf("get legacy limit: %w", err)
	}

	status.Limit = limit

	status.RateLimitCounter = make(map[string]int64)
	status.UsageCounter = make(map[string]int64)

	now := middleware.GetTime(ctx)

	for i := range proto.Service_name {
		svc := proto.Service(i)

		cfg, ok := limit.GetSettings(svc)
		if !ok {
			continue
		}

		cacheKey := cacheKeyQuota(projectID, info.Cycle, &svc, now)
		usage, err := s.cache.UsageCache.PeekUsage(ctx, cacheKey)
		if err != nil {
			if !errors.Is(err, errCacheReady) {
				return nil, fmt.Errorf("peek usage cache: %w", err)
			}
			if _, err := s.PrepareUsage(ctx, projectID, &svc, info.Cycle, now); err != nil {
				return nil, fmt.Errorf("prepare usage: %w", err)
			}
			if usage, err = s.cache.UsageCache.PeekUsage(ctx, cacheKey); err != nil {
				return nil, fmt.Errorf("peek usage cache: %w", err)
			}
		}

		limitCounter := NewLimitCounter(svc, s.redis, s.log)
		limiter := httprate.NewRateLimiter(int(cfg.RateLimit), time.Minute, httprate.WithLimitCounter(limitCounter))
		_, rate, err := limiter.Status(middleware.ProjectRateKey(projectID) + ":")
		if err != nil {
			return nil, fmt.Errorf("get rate limit status: %w", err)
		}
		name := svc.GetName()
		status.UsageCounter[name] = usage
		status.RateLimitCounter[name] = int64(rate)
	}

	return &status, nil
}

func (s server) GetProjectInfo(ctx context.Context, projectId uint64) (*proto.ProjectInfo, error) {
	panic("not implemented")
}

func (s server) ClearProjectInfoCache(ctx context.Context, projectID uint64) (bool, error) {
	panic("not implemented")
}

func (s server) GetServiceLimit(ctx context.Context, projectId uint64, service proto.Service, now time.Time) (*proto.Limit, error) {
	panic("not implemented")
}

func (s server) ClearServiceLimitCache(ctx context.Context, projectID uint64, service proto.Service) (bool, error) {
	panic("not implemented")
}

func (s server) GetAccessKey(ctx context.Context, accessKey string) (*proto.AccessKey, error) {
	return s.store.AccessKeyStore.FindAccessKey(ctx, accessKey)
}

func (s server) ClearAccessKeyCache(ctx context.Context, accessKey string) (bool, error) {
	panic("not implemented")
}
