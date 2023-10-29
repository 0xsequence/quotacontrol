package quotacontrol

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/0xsequence/quotacontrol/proto"
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

func NewQuotaControl(usageCache UsageCache, quotaCache QuotaCache, limit LimitStore, access AccessKeyStore, usage UsageStore) proto.QuotaControl {
	return &quotaControl{
		usageCache:     usageCache,
		quotaCache:     quotaCache,
		limitStore:     limit,
		accessKeyStore: access,
		usageStore:     usage,
		accessKeyGen:   DefaultAccessKey,
	}
}

type quotaControl struct {
	usageCache     UsageCache
	quotaCache     QuotaCache
	limitStore     LimitStore
	accessKeyStore AccessKeyStore
	usageStore     UsageStore
	accessKeyGen   func(projectID uint64) string
}

var _ proto.QuotaControl = &quotaControl{}

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

func (q quotaControl) GetAccountUsage(ctx context.Context, projectID uint64, service *proto.Service, from, to *time.Time) (*proto.AccessUsage, error) {
	min, max := getTimeInterval(from, to)

	usage, err := q.usageStore.GetAccountUsage(ctx, projectID, service, min, max)
	if err != nil {
		return nil, err
	}
	return &usage, nil
}

func (q quotaControl) GetAccessKeyUsage(ctx context.Context, accessKey string, service *proto.Service, from, to *time.Time) (*proto.AccessUsage, error) {
	min, max := getTimeInterval(from, to)

	usage, err := q.usageStore.GetAccessKeyUsage(ctx, accessKey, service, min, max)
	if err != nil {
		return nil, err
	}
	return &usage, nil
}

func (q quotaControl) PrepareUsage(ctx context.Context, projectID uint64, now time.Time) (bool, error) {
	usage, err := q.GetAccountUsage(ctx, projectID, nil, proto.Ptr(firstOfTheMonth(now)), nil)
	if err != nil {
		return false, err
	}
	if err := q.usageCache.SetComputeUnits(ctx, getQuotaKey(projectID, now), usage.GetTotalUsage()); err != nil {
		return false, err
	}
	return true, nil
}

func (q quotaControl) GetAccessQuota(ctx context.Context, accessKey string) (*proto.AccessQuota, error) {
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
	go q.quotaCache.SetAccessQuota(ctx, &record)
	return &record, nil
}

func (q quotaControl) NotifyEvent(ctx context.Context, projectID uint64, eventType *proto.EventType) (bool, error) {
	return true, nil
}

func (q quotaControl) UpdateUsage(ctx context.Context, service *proto.Service, now time.Time, usage map[string]*proto.AccessUsage) (map[string]bool, error) {
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

func (q quotaControl) GetAccessKey(ctx context.Context, accessKey string) (*proto.AccessKey, error) {
	return q.accessKeyStore.FindAccessKey(ctx, accessKey)
}

func (q quotaControl) CreateAccessKey(ctx context.Context, projectID uint64, displayName string, allowedOrigins []string, allowedServices []*proto.Service) (*proto.AccessKey, error) {
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

func (q quotaControl) RotateAccessKey(ctx context.Context, accessKey string) (*proto.AccessKey, error) {
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

func (q quotaControl) UpdateAccessKey(ctx context.Context, accessKey string, displayName *string, allowedOrigins []string, allowedServices []*proto.Service) (*proto.AccessKey, error) {
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

func (q quotaControl) ListAccessKeys(ctx context.Context, projectID uint64, active *bool, service *proto.Service) ([]*proto.AccessKey, error) {
	return q.accessKeyStore.ListAccessKeys(ctx, projectID, active, service)
}

func (q quotaControl) DisableAccessKey(ctx context.Context, accessKey string) (bool, error) {
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

func (q quotaControl) VerifyUserAuthorization(ctx context.Context, accessKey string, userId string) (*proto.UserPermission, map[string]interface{}, error) {
	return nil, nil, nil
}
