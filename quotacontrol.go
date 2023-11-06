package quotacontrol

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/0xsequence/quotacontrol/proto"
	"github.com/goware/logger"
)

type LimitStore interface {
	GetAccessLimit(ctx context.Context, projectID uint64) (*proto.Limit, error)
	SetAccessLimit(ctx context.Context, projectID uint64, config *proto.Limit) error
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
	UpdateAccessUsage(ctx context.Context, accessKey string, service proto.Service, time time.Time, usage proto.AccessUsage) error
}

type PermissionStore interface {
	GetUserPermission(ctx context.Context, projectID uint64, userID string) (*proto.UserPermission, map[string]interface{}, error)
}

// NewQuotaControlHandler returns server implementation for proto.QuotaControl which is used
// by the Builder (aka quotacontrol backend).
func NewQuotaControlHandler(log logger.Logger, usageCache UsageCache, quotaCache QuotaCache, permCache PermissionCache, limit LimitStore, access AccessKeyStore, usage UsageStore, perm PermissionStore) proto.QuotaControl {
	return &qcHandler{
		log:            log,
		usageCache:     usageCache,
		quotaCache:     quotaCache,
		permCache:      permCache,
		limitStore:     limit,
		accessKeyStore: access,
		usageStore:     usage,
		permStore:      perm,
		accessKeyGen:   DefaultAccessKey,
	}
}

// qcHandler is the quotacontrol handler backend implementation.
type qcHandler struct {
	log            logger.Logger
	usageCache     UsageCache
	quotaCache     QuotaCache
	permCache      PermissionCache
	limitStore     LimitStore
	accessKeyStore AccessKeyStore
	usageStore     UsageStore
	permStore      PermissionStore
	accessKeyGen   func(projectID uint64) string
}

var _ proto.QuotaControl = &qcHandler{}

func (q qcHandler) GetAccountUsage(ctx context.Context, projectID uint64, service *proto.Service, from, to *time.Time) (*proto.AccessUsage, error) {
	min, max := getTimeInterval(from, to)

	usage, err := q.usageStore.GetAccountUsage(ctx, projectID, service, min, max)
	if err != nil {
		return nil, err
	}
	return &usage, nil
}

func (q qcHandler) GetAccessKeyUsage(ctx context.Context, accessKey string, service *proto.Service, from, to *time.Time) (*proto.AccessUsage, error) {
	min, max := getTimeInterval(from, to)

	usage, err := q.usageStore.GetAccessKeyUsage(ctx, accessKey, service, min, max)
	if err != nil {
		return nil, err
	}
	return &usage, nil
}

func (q qcHandler) PrepareUsage(ctx context.Context, projectID uint64, now time.Time) (bool, error) {
	usage, err := q.GetAccountUsage(ctx, projectID, nil, proto.Ptr(firstOfTheMonth(now)), nil)
	if err != nil {
		return false, err
	}
	if err := q.usageCache.SetComputeUnits(ctx, getQuotaKey(projectID, now), usage.GetTotalUsage()); err != nil {
		return false, err
	}
	return true, nil
}

func (q qcHandler) GetAccessQuota(ctx context.Context, accessKey string) (*proto.AccessQuota, error) {
	access, err := q.accessKeyStore.FindAccessKey(ctx, accessKey)
	if err != nil {
		return nil, err
	}
	limit, err := q.limitStore.GetAccessLimit(ctx, access.ProjectID)
	if err != nil {
		return nil, err
	}
	record := proto.AccessQuota{
		Limit:     limit,
		AccessKey: access,
	}
	go func() {
		// NOTE: we pass a fresh context here, as otherwise requests/other contexts which cancel
		// above will cancel this goroutine and the AccessQuota will never be saved into cache.
		err := q.quotaCache.SetAccessQuota(context.Background(), &record)
		if err != nil {
			q.log.With("err", err).Error("quotacontrol: failed to set access quota in cache")
		}
	}()
	return &record, nil
}

func (q qcHandler) NotifyEvent(ctx context.Context, projectID uint64, eventType *proto.EventType) (bool, error) {
	return true, nil
}

func (q qcHandler) UpdateUsage(ctx context.Context, service *proto.Service, now time.Time, usage map[string]*proto.AccessUsage) (map[string]bool, error) {
	var errs []error
	m := make(map[string]bool, len(usage))
	for accessKey, accessUsage := range usage {
		err := q.usageStore.UpdateAccessUsage(ctx, accessKey, *service, now, *accessUsage)
		if err != nil {
			errs = append(errs, fmt.Errorf("%s: %w", accessKey, err))
		}
		m[accessKey] = err == nil
	}
	if len(errs) > 0 {
		return m, errors.Join(errs...)
	}
	return m, nil
}

func (q qcHandler) GetAccessLimit(ctx context.Context, projectID uint64) (*proto.Limit, error) {
	return q.limitStore.GetAccessLimit(ctx, projectID)
}

func (q qcHandler) SetAccessLimit(ctx context.Context, projectID uint64, config *proto.Limit) (bool, error) {
	if err := config.Validate(); err != nil {
		return false, proto.WebRPCError{HTTPStatus: http.StatusBadRequest, Message: err.Error()}
	}
	err := q.limitStore.SetAccessLimit(ctx, projectID, config)
	if err != nil {
		return false, err
	}
	return true, nil
}

func (q qcHandler) GetAccessKey(ctx context.Context, accessKey string) (*proto.AccessKey, error) {
	return q.accessKeyStore.FindAccessKey(ctx, accessKey)
}

func (q qcHandler) CreateAccessKey(ctx context.Context, projectID uint64, displayName string, allowedOrigins []string, allowedServices []*proto.Service) (*proto.AccessKey, error) {
	limit, err := q.limitStore.GetAccessLimit(ctx, projectID)
	if err != nil {
		return nil, err
	}
	if limit.MaxKeys > 0 {
		list, err := q.accessKeyStore.ListAccessKeys(ctx, projectID, proto.Ptr(true), nil)
		if err != nil {
			return nil, err
		}
		if l := len(list); int64(l) >= limit.MaxKeys {
			return nil, proto.ErrMaxAccessKeys
		}
	}

	access := proto.AccessKey{
		ProjectID:       projectID,
		DisplayName:     displayName,
		AccessKey:       q.accessKeyGen(projectID),
		Active:          true,
		AllowedOrigins:  allowedOrigins,
		AllowedServices: allowedServices,
	}
	if err := q.accessKeyStore.InsertAccessKey(ctx, &access); err != nil {
		return nil, err
	}
	return &access, nil
}

func (q qcHandler) RotateAccessKey(ctx context.Context, accessKey string) (*proto.AccessKey, error) {
	access, err := q.accessKeyStore.FindAccessKey(ctx, accessKey)
	if err != nil {
		return nil, err
	}
	access.Active = false
	if _, err := q.accessKeyStore.UpdateAccessKey(ctx, access); err != nil {
		return nil, err
	}
	return q.CreateAccessKey(ctx, access.ProjectID, access.DisplayName, access.AllowedOrigins, access.AllowedServices)
}

func (q qcHandler) UpdateAccessKey(ctx context.Context, accessKey string, displayName *string, allowedOrigins []string, allowedServices []*proto.Service) (*proto.AccessKey, error) {
	access, err := q.accessKeyStore.FindAccessKey(ctx, accessKey)
	if err != nil {
		return nil, err
	}
	if displayName != nil {
		access.DisplayName = *displayName
	}
	if allowedOrigins != nil {
		access.AllowedOrigins = allowedOrigins
	}
	if allowedServices != nil {
		access.AllowedServices = allowedServices
	}
	if access, err = q.accessKeyStore.UpdateAccessKey(ctx, access); err != nil {
		return nil, err
	}
	return access, nil
}

func (q qcHandler) ListAccessKeys(ctx context.Context, projectID uint64, active *bool, service *proto.Service) ([]*proto.AccessKey, error) {
	return q.accessKeyStore.ListAccessKeys(ctx, projectID, active, service)
}

func (q qcHandler) DisableAccessKey(ctx context.Context, accessKey string) (bool, error) {
	access, err := q.accessKeyStore.FindAccessKey(ctx, accessKey)
	if err != nil {
		return false, err
	}
	access.Active = false
	if _, err := q.accessKeyStore.UpdateAccessKey(ctx, access); err != nil {
		return false, err
	}
	return true, nil
}

func (q qcHandler) GetUserPermission(ctx context.Context, projectID uint64, userID string) (*proto.UserPermission, map[string]interface{}, error) {
	userPerm, resourceAccess, err := q.permStore.GetUserPermission(ctx, projectID, userID)
	if err != nil {
		return userPerm, resourceAccess, proto.ErrUnauthorizedUser
	}
	if userPerm != nil && *userPerm == proto.UserPermission_UNAUTHORIZED {
		return userPerm, resourceAccess, proto.ErrUnauthorizedUser
	}
	go func() {
		// NOTE: we pass a fresh context here, as otherwise requests/other contexts which cancel
		// above will cancel this goroutine and the AccessQuota will never be saved into cache.
		err := q.permCache.SetUserPermission(context.Background(), projectID, userID, userPerm, resourceAccess)
		if err != nil {
			q.log.With("err", err).Error("quotacontrol: failed to set user perm in cache")
		}
	}()

	return userPerm, resourceAccess, nil
}

func firstOfTheMonth(now time.Time) time.Time {
	return time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())
}

func getTimeInterval(from, to *time.Time) (min time.Time, max time.Time) {
	if from == nil {
		from = proto.Ptr(firstOfTheMonth(time.Now()))
	}
	// if not set, one month after `from`
	if to == nil {
		to = proto.Ptr(from.AddDate(0, 1, -1))
	}
	return *from, *to
}
