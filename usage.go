package quotacontrol

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/0xsequence/quotacontrol/proto"
)

// UsageUpdater is an interface that allows to update the usage of a service
type UsageUpdater interface {
	UpdateKeyUsage(ctx context.Context, service *proto.Service, now time.Time, usage map[string]*proto.AccessUsage) (map[string]bool, error)
	UpdateProjectUsage(ctx context.Context, service *proto.Service, now time.Time, usage map[uint64]*proto.AccessUsage) (map[uint64]bool, error)
}

func newUsageRecord() usageRecord {
	return usageRecord{
		ByProjectID: make(map[uint64]*proto.AccessUsage),
		ByAccessKey: make(map[string]*proto.AccessUsage),
	}
}

type usageRecord struct {
	ByProjectID map[uint64]*proto.AccessUsage
	ByAccessKey map[string]*proto.AccessUsage
}

// UsageChanges keeps track of the usage of a service
type usageTracker struct {
	// Mutex used for usage data
	DataMutex sync.Mutex
	// Mutext used for sync (calling Stop while another sync is running will wait for the sync to finish)
	SyncMutex sync.Mutex

	Usage map[time.Time]usageRecord
}

func (u *usageTracker) ensureTime(now time.Time) {
	u.DataMutex.Lock()
	if _, ok := u.Usage[now]; !ok {
		u.Usage[now] = newUsageRecord()
	}
	u.DataMutex.Unlock()
}

func (u *usageTracker) ensureKey(now time.Time, key string) {
	u.ensureTime(now)

	u.DataMutex.Lock()
	if _, ok := u.Usage[now].ByAccessKey[key]; !ok {
		u.Usage[now].ByAccessKey[key] = &proto.AccessUsage{}
	}
	u.DataMutex.Unlock()
}

func (u *usageTracker) ensureProject(now time.Time, projectID uint64) {
	u.ensureTime(now)

	u.DataMutex.Lock()
	if _, ok := u.Usage[now].ByProjectID[projectID]; !ok {
		u.Usage[now].ByProjectID[projectID] = &proto.AccessUsage{}
	}
	u.DataMutex.Unlock()
}

// AddUsage adds the usage of a access key.
func (u *usageTracker) AddKeyUsage(accessKey string, now time.Time, usage proto.AccessUsage) {
	u.ensureTime(now)
	u.ensureKey(now, accessKey)

	u.DataMutex.Lock()
	u.Usage[now].ByAccessKey[accessKey].Add(usage)
	u.DataMutex.Unlock()
}

// AddUsage adds the usage of a access key.
func (u *usageTracker) AddProjectUsage(projectID uint64, now time.Time, usage proto.AccessUsage) {
	u.ensureTime(now)
	u.ensureProject(now, projectID)

	u.DataMutex.Lock()
	u.Usage[now].ByProjectID[projectID].Add(usage)
	u.DataMutex.Unlock()
}

// GetUpdates returns the usage of a service and clears the usage
func (u *usageTracker) GetUpdates() map[time.Time]usageRecord {
	u.DataMutex.Lock()
	result := u.Usage
	u.Usage = make(map[time.Time]usageRecord)
	u.DataMutex.Unlock()
	return result
}

// SyncUsage syncs the usage of a service with the UsageUpdater
func (u *usageTracker) SyncUsage(ctx context.Context, updater UsageUpdater, service *proto.Service) error {
	u.SyncMutex.Lock()
	defer u.SyncMutex.Unlock()
	var errList []error
	for now, usages := range u.GetUpdates() {
		keyResult, err := updater.UpdateKeyUsage(ctx, service, now, usages.ByAccessKey)
		if err != nil {
			errList = append(errList, err)
		}
		// add back to the counter failed updates
		for accessKey, v := range keyResult {
			if v {
				continue
			}
			u.AddKeyUsage(accessKey, now, *usages.ByAccessKey[accessKey])
		}

		projectResult, err := updater.UpdateProjectUsage(ctx, service, now, usages.ByProjectID)
		if err != nil {
			errList = append(errList, err)
		}
		// add back to the counter failed updates
		for projectID, v := range projectResult {
			if v {
				continue
			}
			u.AddProjectUsage(projectID, now, *usages.ByProjectID[projectID])
		}

	}
	return errors.Join(errList...)
}
