package quotacontrol

import (
	"context"
	"sync"
	"time"

	"github.com/0xsequence/quotacontrol/proto"
)

// UsageUptader is an interface that allows to update the usage of a service
type UsageUpdater interface {
	UpdateUsage(ctx context.Context, service *proto.Service, now time.Time, usage map[string]*proto.AccessTokenUsage) (map[string]bool, error)
}

// UsageChanges keeps track of the usage of a service
type usageTracker struct {
	sync.Mutex
	Usage map[time.Time]map[string]*proto.AccessTokenUsage
}

// AddUsage adds the usage of a token.
func (u *usageTracker) AddUsage(tokenKey string, now time.Time, usage proto.AccessTokenUsage) {
	u.Lock()
	defer u.Unlock()
	if _, ok := u.Usage[now]; !ok {
		u.Usage[now] = make(map[string]*proto.AccessTokenUsage)
	}
	if _, ok := u.Usage[now][tokenKey]; !ok {
		u.Usage[now][tokenKey] = &proto.AccessTokenUsage{}
	}
	u.Usage[now][tokenKey].Add(usage)
}

// sync
func (u *usageTracker) SyncUsage(ctx context.Context, updater UsageUpdater, service *proto.Service) error {
	u.Lock()
	tokenUsage := make(map[time.Time]map[string]*proto.AccessTokenUsage, len(u.Usage))
	for k, v := range u.Usage {
		tokenUsage[k] = make(map[string]*proto.AccessTokenUsage, len(v))
		for k1, v1 := range v {
			tokenUsage[k][k1] = v1
		}
		delete(u.Usage, k)
	}
	u.Unlock()
	for now, usages := range tokenUsage {
		result, err := updater.UpdateUsage(ctx, service, now, usages)
		// check if any update failed and add it back to the counter
		for tokenKey, v := range result {
			if v {
				continue
			}
			u.AddUsage(tokenKey, now, *usages[tokenKey])
		}
		if err != nil {
			return err
		}
	}
	return nil
}
