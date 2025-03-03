package usage

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/0xsequence/quotacontrol/proto"
)

// UsageUpdater is an interface that allows to update the usage of a service
type UsageUpdater interface {
	UpdateKeyUsage(ctx context.Context, service proto.Service, now time.Time, usage map[string]*proto.AccessUsage) (map[string]bool, error)
	UpdateProjectUsage(ctx context.Context, service proto.Service, now time.Time, usage map[uint64]*proto.AccessUsage) (map[uint64]bool, error)
}

func NewRecord() Record {
	return Record{
		ByProjectID: make(map[uint64]*proto.AccessUsage),
		ByAccessKey: make(map[string]*proto.AccessUsage),
	}
}

type Record struct {
	ByProjectID map[uint64]*proto.AccessUsage
	ByAccessKey map[string]*proto.AccessUsage
}

func NewTracker() *Tracker {
	return &Tracker{
		usage: make(map[time.Time]Record),
	}
}

// UsageChanges keeps track of the usage of a service
type Tracker struct {
	// Mutex used for usage data
	dataMutex sync.Mutex
	// Mutext used for sync (calling Stop while another sync is running will wait for the sync to finish)
	syncMutex sync.Mutex

	usage map[time.Time]Record
}

// AddUsage adds the usage of a access key.
func (u *Tracker) AddKeyUsage(accessKey string, now time.Time, usage proto.AccessUsage) {
	u.dataMutex.Lock()
	if _, ok := u.usage[now]; !ok {
		u.usage[now] = NewRecord()
	}
	if _, ok := u.usage[now].ByAccessKey[accessKey]; !ok {
		u.usage[now].ByAccessKey[accessKey] = &proto.AccessUsage{}
	}
	u.usage[now].ByAccessKey[accessKey].Add(usage)
	u.dataMutex.Unlock()
}

// AddUsage adds the usage of a access key.
func (u *Tracker) AddProjectUsage(projectID uint64, now time.Time, usage proto.AccessUsage) {
	u.dataMutex.Lock()
	if _, ok := u.usage[now]; !ok {
		u.usage[now] = NewRecord()
	}
	if _, ok := u.usage[now].ByProjectID[projectID]; !ok {
		u.usage[now].ByProjectID[projectID] = &proto.AccessUsage{}
	}
	u.usage[now].ByProjectID[projectID].Add(usage)
	u.dataMutex.Unlock()
}

// GetUpdates returns the usage of a service and clears the usage
func (u *Tracker) GetUpdates() map[time.Time]Record {
	u.dataMutex.Lock()
	result := u.usage
	u.usage = make(map[time.Time]Record)
	u.dataMutex.Unlock()
	return result
}

// SyncUsage syncs the usage of a service with the UsageUpdater
func (u *Tracker) SyncUsage(ctx context.Context, updater UsageUpdater, service proto.Service) error {
	u.syncMutex.Lock()
	defer u.syncMutex.Unlock()
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
