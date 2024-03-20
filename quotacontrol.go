package quotacontrol

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/0xsequence/quotacontrol/middleware"
	"github.com/0xsequence/quotacontrol/proto"
	"github.com/go-chi/httprate"
	"github.com/goware/logger"
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
	GetAccessKeyUsage(ctx context.Context, accessKey string, service *proto.Service, min, max time.Time) (proto.AccessUsage, error)
	GetAccountUsage(ctx context.Context, projectID uint64, service *proto.Service, min, max time.Time) (proto.AccessUsage, error)
	UpdateAccessUsage(ctx context.Context, projectID uint64, accessKey string, service proto.Service, time time.Time, usage proto.AccessUsage) error
}

type CycleStore interface {
	GetAccessCycle(ctx context.Context, projectID uint64, now time.Time) (*proto.Cycle, error)
}

type PermissionStore interface {
	GetUserPermission(ctx context.Context, projectID uint64, userID string) (*proto.UserPermission, *proto.ResourceAccess, error)
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

// NewQuotaControlHandler returns server implementation for proto.QuotaControl which is used
// by the Builder (aka quotacontrol backend).
func NewQuotaControlHandler(log logger.Logger, cache Cache, store Store, limitcounter httprate.LimitCounter) proto.QuotaControl {
	return &qcHandler{
		log:          log,
		cache:        cache,
		store:        store,
		limitCounter: limitcounter,
		accessKeyGen: GenerateAccessKey,
	}
}

// qcHandler is the quotacontrol handler backend implementation.
type qcHandler struct {
	log          logger.Logger
	cache        Cache
	store        Store
	limitCounter httprate.LimitCounter
	accessKeyGen func(projectID uint64) string
}

var _ proto.QuotaControl = &qcHandler{}

func (q qcHandler) GetTimeRange(ctx context.Context, projectID uint64, from, to *time.Time) (time.Time, time.Time, error) {
	if from != nil && to != nil {
		return *from, *to, nil
	}
	now := middleware.GetTime(ctx)
	cycle, err := q.store.CycleStore.GetAccessCycle(ctx, projectID, now)
	if err != nil {
		return time.Time{}, time.Time{}, err
	}
	if from == nil && to == nil {
		cycle, _ := DefaultCycleStore{}.GetAccessCycle(ctx, projectID, now)
		return cycle.GetStart(now), cycle.GetEnd(now), nil
	}
	duration := cycle.GetDuration(now)
	if from == nil {
		return to.Add(-duration), *to, nil
	}
	return *from, from.Add(duration), nil
}

func (q qcHandler) GetAccountUsage(ctx context.Context, projectID uint64, service *proto.Service, from, to *time.Time) (*proto.AccessUsage, error) {
	min, max, err := q.GetTimeRange(ctx, projectID, from, to)
	if err != nil {
		return nil, err
	}

	usage, err := q.store.UsageStore.GetAccountUsage(ctx, projectID, service, min, max)
	if err != nil {
		return nil, err
	}
	return &usage, nil
}

func (q qcHandler) GetAccessKeyUsage(ctx context.Context, accessKey string, service *proto.Service, from, to *time.Time) (*proto.AccessUsage, error) {
	projectID, err := GetProjectID(accessKey)
	if err != nil {
		return nil, err
	}

	min, max, err := q.GetTimeRange(ctx, projectID, from, to)
	if err != nil {
		return nil, err
	}

	usage, err := q.store.UsageStore.GetAccessKeyUsage(ctx, accessKey, service, min, max)
	if err != nil {
		return nil, err
	}
	return &usage, nil
}

func (q qcHandler) PrepareUsage(ctx context.Context, projectID uint64, cycle *proto.Cycle, now time.Time) (bool, error) {
	min, max := cycle.GetStart(now), cycle.GetEnd(now)
	usage, err := q.GetAccountUsage(ctx, projectID, nil, &min, &max)
	if err != nil {
		return false, err
	}
	key := getQuotaKey(projectID, cycle, now)
	if err := q.cache.UsageCache.SetComputeUnits(ctx, key, usage.GetTotalUsage()); err != nil {
		return false, err
	}
	return true, nil
}

func (q qcHandler) ClearUsage(ctx context.Context, projectID uint64, now time.Time) (bool, error) {
	cycle, err := q.store.CycleStore.GetAccessCycle(ctx, projectID, now)
	if err != nil {
		return false, err
	}

	key := getQuotaKey(projectID, cycle, now)
	ok, err := q.cache.UsageCache.ClearComputeUnits(ctx, key)
	if err != nil {
		return false, err
	}
	return ok, nil
}

func (q qcHandler) GetProjectQuota(ctx context.Context, projectID uint64, now time.Time) (*proto.AccessQuota, error) {
	cycle, err := q.store.CycleStore.GetAccessCycle(ctx, projectID, now)
	if err != nil {
		return nil, err
	}

	limit, err := q.store.LimitStore.GetAccessLimit(ctx, projectID, cycle)
	if err != nil {
		return nil, err
	}

	record := proto.AccessQuota{
		Limit:     limit,
		Cycle:     cycle,
		AccessKey: &proto.AccessKey{ProjectID: projectID},
	}

	if err := q.cache.QuotaCache.SetProjectQuota(ctx, &record); err != nil {
		q.log.With("err", err).Error("quotacontrol: failed to set access quota in cache")
	}

	return &record, nil
}
func (q qcHandler) GetAccessQuota(ctx context.Context, accessKey string, now time.Time) (*proto.AccessQuota, error) {
	access, err := q.store.AccessKeyStore.FindAccessKey(ctx, accessKey)
	if err != nil {
		return nil, err
	}
	cycle, err := q.store.CycleStore.GetAccessCycle(ctx, access.ProjectID, now)
	if err != nil {
		return nil, err
	}
	limit, err := q.store.LimitStore.GetAccessLimit(ctx, access.ProjectID, cycle)
	if err != nil {
		return nil, err
	}
	record := proto.AccessQuota{
		Limit:     limit,
		Cycle:     cycle,
		AccessKey: access,
	}

	if err := q.cache.QuotaCache.SetAccessQuota(ctx, &record); err != nil {
		q.log.With("err", err).Error("quotacontrol: failed to set access quota in cache")
	}

	return &record, nil
}

func (q qcHandler) NotifyEvent(ctx context.Context, projectID uint64, eventType *proto.EventType) (bool, error) {
	return true, nil
}

func (q qcHandler) UpdateProjectUsage(ctx context.Context, service *proto.Service, now time.Time, usage map[uint64]*proto.AccessUsage) (map[uint64]bool, error) {
	var errs []error
	m := make(map[uint64]bool, len(usage))
	for projectID, accessUsage := range usage {
		err := q.store.UsageStore.UpdateAccessUsage(ctx, projectID, "", *service, now, *accessUsage)
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

func (q qcHandler) UpdateKeyUsage(ctx context.Context, service *proto.Service, now time.Time, usage map[string]*proto.AccessUsage) (map[string]bool, error) {
	var errs []error
	m := make(map[string]bool, len(usage))
	for key, u := range usage {
		projectID, err := GetProjectID(key)
		if err != nil {
			errs = append(errs, fmt.Errorf("%s: %w", key, err))
			continue
		}
		if err = q.store.UsageStore.UpdateAccessUsage(ctx, projectID, key, *service, now, *u); err != nil {
			errs = append(errs, fmt.Errorf("%s: %w", key, err))
		}
		m[key] = err == nil
	}
	if len(errs) > 0 {
		return m, errors.Join(errs...)
	}
	return m, nil
}

func (q qcHandler) UpdateUsage(ctx context.Context, service *proto.Service, now time.Time, usage map[string]*proto.AccessUsage) (map[string]bool, error) {
	return q.UpdateKeyUsage(ctx, service, now, usage)
}

func (q qcHandler) ClearAccessQuotaCache(ctx context.Context, projectID uint64) (bool, error) {
	accessKeys, err := q.ListAccessKeys(ctx, projectID, proto.Ptr(true), nil)
	if err != nil {
		q.log.With("err", err).Error("quotacontrol: failed to list access keys")
		return true, nil
	}
	if err := q.cache.QuotaCache.DeleteProjectQuota(ctx, projectID); err != nil {
		q.log.With("err", err).Error("quotacontrol: failed to delete access quota from cache")
	}
	for _, access := range accessKeys {
		if err := q.cache.QuotaCache.DeleteAccessQuota(ctx, access.AccessKey); err != nil {
			q.log.With("err", err).Error("quotacontrol: failed to delete access quota from cache")
		}
	}
	return true, nil
}

func (q qcHandler) GetAccessKey(ctx context.Context, accessKey string) (*proto.AccessKey, error) {
	return q.store.AccessKeyStore.FindAccessKey(ctx, accessKey)
}

func (q qcHandler) GetDefaultAccessKey(ctx context.Context, projectID uint64) (*proto.AccessKey, error) {
	list, err := q.store.AccessKeyStore.ListAccessKeys(ctx, projectID, proto.Ptr(true), nil)
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

func (q qcHandler) CreateAccessKey(ctx context.Context, projectID uint64, displayName string, allowedOrigins []string, allowedServices []*proto.Service) (*proto.AccessKey, error) {
	cycle, err := q.store.CycleStore.GetAccessCycle(ctx, projectID, middleware.GetTime(ctx))
	if err != nil {
		return nil, err
	}
	limit, err := q.store.LimitStore.GetAccessLimit(ctx, projectID, cycle)
	if err != nil {
		return nil, err
	}

	list, err := q.store.AccessKeyStore.ListAccessKeys(ctx, projectID, proto.Ptr(true), nil)
	if err != nil {
		return nil, err
	}

	if limit.MaxKeys > 0 {
		if l := len(list); int64(l) >= limit.MaxKeys {
			return nil, proto.ErrMaxAccessKeys
		}
	}

	origins := make([]validation.Origin, len(allowedOrigins))
	for i, o := range allowedOrigins {
		origin, err := validation.NewOrigin(o)
		if err != nil {
			return nil, err
		}
		origins[i] = origin
	}

	access := proto.AccessKey{
		ProjectID:       projectID,
		DisplayName:     displayName,
		AccessKey:       q.accessKeyGen(projectID),
		Active:          true,
		Default:         len(list) == 0,
		AllowedOrigins:  origins,
		AllowedServices: allowedServices,
	}
	if err := q.store.AccessKeyStore.InsertAccessKey(ctx, &access); err != nil {
		return nil, err
	}
	return &access, nil
}

func (q qcHandler) RotateAccessKey(ctx context.Context, accessKey string) (*proto.AccessKey, error) {
	access, err := q.store.AccessKeyStore.FindAccessKey(ctx, accessKey)
	if err != nil {
		return nil, err
	}

	isDefaultKey := access.Default

	access.Active = false
	access.Default = false

	if _, err := q.updateAccessKey(ctx, access); err != nil {
		return nil, err
	}

	origins := make([]string, len(access.AllowedOrigins))
	for i, origin := range access.AllowedOrigins {
		origins[i] = string(origin)
	}

	newAccess, err := q.CreateAccessKey(ctx, access.ProjectID, access.DisplayName, origins, access.AllowedServices)
	if err != nil {
		return nil, err
	}

	if isDefaultKey {
		// set new key as default
		newAccess.Default = true
		return q.updateAccessKey(ctx, newAccess)
	}

	return newAccess, nil
}

func (q qcHandler) UpdateAccessKey(ctx context.Context, accessKey string, displayName *string, allowedOrigins []string, allowedServices []*proto.Service) (*proto.AccessKey, error) {
	access, err := q.store.AccessKeyStore.FindAccessKey(ctx, accessKey)
	if err != nil {
		return nil, err
	}

	if displayName != nil {
		access.DisplayName = *displayName
	}
	if allowedOrigins != nil {
		origins := make([]validation.Origin, len(allowedOrigins))
		for i, o := range allowedOrigins {
			origin, err := validation.NewOrigin(o)
			if err != nil {
				return nil, err
			}
			origins[i] = origin
		}
		access.AllowedOrigins = origins
	}
	if allowedServices != nil {
		access.AllowedServices = allowedServices
	}

	if access, err = q.updateAccessKey(ctx, access); err != nil {
		return nil, err
	}
	return access, nil
}

func (q qcHandler) UpdateDefaultAccessKey(ctx context.Context, projectID uint64, accessKey string) (bool, error) {
	// make sure accessKey exists
	access, err := q.store.AccessKeyStore.FindAccessKey(ctx, accessKey)
	if err != nil {
		return false, err
	}

	if access.ProjectID != projectID {
		return false, fmt.Errorf("project doesn't own the given access key")
	}

	defaultAccess, err := q.GetDefaultAccessKey(ctx, projectID)
	if err != nil {
		return false, err
	}

	// make sure new default access key & old default access key are different
	if defaultAccess.AccessKey == access.AccessKey {
		return true, nil
	}

	// update old default access
	defaultAccess.Default = false
	if _, err := q.updateAccessKey(ctx, defaultAccess); err != nil {
		return false, err
	}

	// set new access key to default
	access.Default = true
	if _, err = q.updateAccessKey(ctx, access); err != nil {
		return false, err
	}

	return true, nil
}

func (q qcHandler) ListAccessKeys(ctx context.Context, projectID uint64, active *bool, service *proto.Service) ([]*proto.AccessKey, error) {
	return q.store.AccessKeyStore.ListAccessKeys(ctx, projectID, active, service)
}

func (q qcHandler) DisableAccessKey(ctx context.Context, accessKey string) (bool, error) {
	access, err := q.store.AccessKeyStore.FindAccessKey(ctx, accessKey)
	if err != nil {
		return false, err
	}

	list, err := q.store.AccessKeyStore.ListAccessKeys(ctx, access.ProjectID, proto.Ptr(true), nil)
	if err != nil {
		return false, err
	}

	if len(list) == 1 {
		return false, proto.ErrAtLeastOneKey
	}

	access.Active = false
	access.Default = false
	if _, err := q.updateAccessKey(ctx, access); err != nil {
		return false, err
	}

	// set another project accessKey to default
	if _, err := q.GetDefaultAccessKey(ctx, access.ProjectID); err == proto.ErrNoDefaultKey {
		listUpdated, err := q.store.AccessKeyStore.ListAccessKeys(ctx, access.ProjectID, proto.Ptr(true), nil)
		if err != nil {
			return false, err
		}

		newDefaultKey := listUpdated[0]
		newDefaultKey.Default = true

		if _, err = q.updateAccessKey(ctx, newDefaultKey); err != nil {
			return false, err
		}
	}

	return true, nil
}

func (q qcHandler) GetUserPermission(ctx context.Context, projectID uint64, userID string) (*proto.UserPermission, *proto.ResourceAccess, error) {
	perm, access, err := q.store.PermissionStore.GetUserPermission(ctx, projectID, userID)
	if err != nil {
		return perm, access, proto.ErrUnauthorizedUser
	}
	if perm != nil && *perm == proto.UserPermission_UNAUTHORIZED {
		return perm, access, proto.ErrUnauthorizedUser
	}

	if err := q.cache.PermissionCache.SetUserPermission(ctx, projectID, userID, perm, access); err != nil {
		q.log.With("err", err).Error("quotacontrol: failed to set user perm in cache")
	}

	return perm, access, nil
}

func (q qcHandler) updateAccessKey(ctx context.Context, access *proto.AccessKey) (*proto.AccessKey, error) {
	access, err := q.store.AccessKeyStore.UpdateAccessKey(ctx, access)
	if err != nil {
		return nil, err
	}

	if err := q.cache.QuotaCache.DeleteAccessQuota(ctx, access.AccessKey); err != nil {
		q.log.With("err", err).Error("quotacontrol: failed to delete access quota from cache")
	}

	return access, nil
}

func (q qcHandler) GetProjectStatus(ctx context.Context, projectID uint64) (*proto.ProjectStatus, error) {
	status := proto.ProjectStatus{
		ProjectID: projectID,
	}

	now := middleware.GetTime(ctx)
	cycle, err := q.store.CycleStore.GetAccessCycle(ctx, projectID, now)
	if err != nil {
		return nil, err
	}

	limit, err := q.store.LimitStore.GetAccessLimit(ctx, projectID, cycle)
	if err != nil {
		return nil, err
	}
	status.Limit = limit

	key := getQuotaKey(projectID, cycle, now)

	usage, err := q.cache.UsageCache.PeekComputeUnits(ctx, key)
	if err != nil {
		if !errors.Is(err, ErrCachePing) {
			return nil, err
		}
		if _, err := q.PrepareUsage(ctx, projectID, cycle, now); err != nil {
			return nil, err
		}
		if usage, err = q.cache.UsageCache.PeekComputeUnits(ctx, key); err != nil {
			return nil, err
		}
	}
	status.UsageCounter = usage

	limiter := httprate.NewRateLimiter(int(limit.RateLimit), time.Minute, httprate.WithLimitCounter(q.limitCounter))
	_, rate, err := limiter.Status(middleware.ProjectRateKey(projectID) + ":")
	if err != nil {
		return nil, err
	}
	status.RatelimitCounter = int64(rate)

	return &status, nil
}
