package quotacontrol

import (
	"context"
	"time"

	"github.com/0xsequence/quotacontrol/proto"
)

// NewMemoryStore returns a new in-memory store.
func NewMemoryStore() *MemoryStore {
	return &MemoryStore{
		limits:     map[uint64]*proto.Limit{},
		accessKeys: map[string]*proto.AccessKey{},
		usage:      map[string]*proto.AccessUsage{},
	}
}

// MemoryStore is an in-memory store, used for testing and prototype.
type MemoryStore struct {
	limits     map[uint64]*proto.Limit
	accessKeys map[string]*proto.AccessKey
	usage      map[string]*proto.AccessUsage
}

func (m MemoryStore) SetAccessLimit(ctx context.Context, projectID uint64, config *proto.Limit) error {
	m.limits[projectID] = config
	return nil
}

func (m MemoryStore) GetAccessLimit(ctx context.Context, projectID uint64) (*proto.Limit, error) {
	limit, ok := m.limits[projectID]
	if !ok {
		return nil, proto.ErrAccessKeyNotFound
	}
	return limit, nil
}

func (m MemoryStore) InsertAccessKey(ctx context.Context, access *proto.AccessKey) error {
	m.accessKeys[access.AccessKey] = access
	return nil
}

func (m MemoryStore) UpdateAccessKey(ctx context.Context, access *proto.AccessKey) (*proto.AccessKey, error) {
	m.accessKeys[access.AccessKey] = access
	return access, nil
}

func (m MemoryStore) FindAccessKey(ctx context.Context, accessKey string) (*proto.AccessKey, error) {
	access, ok := m.accessKeys[accessKey]
	if !ok {
		return nil, proto.ErrAccessKeyNotFound
	}
	return access, nil
}

func (m MemoryStore) ListAccessKeys(ctx context.Context, projectID uint64, active *bool) ([]*proto.AccessKey, error) {
	accessKeys := []*proto.AccessKey{}
	for i, v := range m.accessKeys {
		if v.ProjectID == projectID {
			accessKeys = append(accessKeys, m.accessKeys[i])
		}
	}
	return accessKeys, nil
}

func (m MemoryStore) GetAccountTotalUsage(ctx context.Context, projectID uint64, service *proto.Service, min, max time.Time) (proto.AccessUsage, error) {
	usage := proto.AccessUsage{}
	for _, v := range m.accessKeys {
		if v.ProjectID == projectID {
			u, ok := m.usage[v.AccessKey]
			if !ok {
				continue
			}
			usage.Add(*u)
		}
	}
	return usage, nil
}

func (m MemoryStore) UpdateAccessUsage(ctx context.Context, accessKey string, service proto.Service, time time.Time, usage proto.AccessUsage) error {
	if _, ok := m.usage[accessKey]; !ok {
		m.usage[accessKey] = &usage
		return nil
	}
	m.usage[accessKey].Add(usage)
	return nil
}

func (m MemoryStore) ResetUsage(ctx context.Context, accessKey string, service proto.Service) error {
	m.usage[accessKey] = &proto.AccessUsage{}
	return nil
}
