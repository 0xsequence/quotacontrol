package quotacontrol

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/0xsequence/quotacontrol/proto"
)

// UsageUpdater is an interface that allows to update the usage of a service
type UsageUpdater interface {
	UpdateUsage(ctx context.Context, service *proto.Service, now time.Time, usage map[string]*proto.AccessTokenUsage) (map[string]bool, error)
}

// UsageChanges keeps track of the usage of a service
type usageTracker struct {
	// Mutex used for usage data
	DataMutex sync.Mutex
	// Mutext used for sync (calling Stop while another sync is running will wait for the sync to finish)
	SyncMutex sync.Mutex

	Usage map[time.Time]map[string]*proto.AccessTokenUsage
}

// AddUsage adds the usage of a token.
func (u *usageTracker) AddUsage(tokenKey string, now time.Time, usage proto.AccessTokenUsage) {
	u.DataMutex.Lock()
	defer u.DataMutex.Unlock()
	if _, ok := u.Usage[now]; !ok {
		u.Usage[now] = make(map[string]*proto.AccessTokenUsage)
	}
	if _, ok := u.Usage[now][tokenKey]; !ok {
		u.Usage[now][tokenKey] = &proto.AccessTokenUsage{}
	}
	u.Usage[now][tokenKey].Add(usage)
}

// GetUpdates returns the usage of a service and clears the usage
func (u *usageTracker) GetUpdates() map[time.Time]map[string]*proto.AccessTokenUsage {
	u.DataMutex.Lock()
	defer u.DataMutex.Unlock()
	result := make(map[time.Time]map[string]*proto.AccessTokenUsage, len(u.Usage))
	for k, v := range u.Usage {
		result[k] = make(map[string]*proto.AccessTokenUsage, len(v))
		for k1, v1 := range v {
			result[k][k1] = v1
		}
		delete(u.Usage, k)
	}
	return result
}

// SyncUsage syncs the usage of a service with the UsageUpdater
func (u *usageTracker) SyncUsage(ctx context.Context, updater UsageUpdater, service *proto.Service) error {
	u.SyncMutex.Lock()
	defer u.SyncMutex.Unlock()
	var errList []error
	for now, usages := range u.GetUpdates() {
		for k, v := range usages {
			fmt.Println("sync usage", k, now, v)
		}
		result, err := updater.UpdateUsage(ctx, service, now, usages)
		if err != nil {
			errList = append(errList, err)
		}
		// add back to the counter failed updates
		for tokenKey, v := range result {
			if v {
				continue
			}
			u.AddUsage(tokenKey, now, *usages[tokenKey])
		}
	}
	return errors.Join(errList...)
}
