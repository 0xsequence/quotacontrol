package quotacontrol

import (
	"context"
	"sync"
	"time"

	"github.com/0xsequence/quotacontrol/proto"
)

// NewMemoryStore returns a new in-memory store.
func NewMemoryStore() *MemoryStore {
	return &MemoryStore{
		limits:     map[uint64]*proto.Limit{},
		accessKeys: map[string]*proto.AccessKey{},
		usage: usageRecord{
			ByProjectID: map[uint64]*proto.AccessUsage{},
			ByAccessKey: map[string]*proto.AccessUsage{},
		},
	}
}

// MemoryStore is an in-memory store, used for testing and prototype.
type MemoryStore struct {
	sync.Mutex
	limits     map[uint64]*proto.Limit
	cycles     map[uint64]*proto.Cycle
	accessKeys map[string]*proto.AccessKey
	usage      usageRecord
}

func (m *MemoryStore) SetAccessLimit(ctx context.Context, projectID uint64, config *proto.Limit) error {
	m.Lock()
	m.limits[projectID] = config
	m.Unlock()
	return nil
}

func (m *MemoryStore) GetAccessLimit(ctx context.Context, projectID uint64) (*proto.Limit, error) {
	m.Lock()
	limit, ok := m.limits[projectID]
	m.Unlock()
	if !ok {
		return nil, proto.ErrAccessKeyNotFound
	}
	return limit, nil
}

func (m *MemoryStore) SetAccessCycle(ctx context.Context, projectID uint64, cycle *proto.Cycle) error {
	m.Lock()
	m.cycles[projectID] = cycle
	m.Unlock()
	return nil
}

func (m *MemoryStore) GetAccessCycle(ctx context.Context, projectID uint64, now time.Time) (*proto.Cycle, error) {
	m.Lock()
	cycle := m.cycles[projectID]
	m.Unlock()
	return cycle, nil
}

func (m *MemoryStore) InsertAccessKey(ctx context.Context, access *proto.AccessKey) error {
	m.Lock()
	m.accessKeys[access.AccessKey] = access
	m.Unlock()
	return nil
}

func (m *MemoryStore) UpdateAccessKey(ctx context.Context, access *proto.AccessKey) (*proto.AccessKey, error) {
	m.Lock()
	m.accessKeys[access.AccessKey] = access
	m.Unlock()
	return access, nil
}

func (m *MemoryStore) FindAccessKey(ctx context.Context, accessKey string) (*proto.AccessKey, error) {
	m.Lock()
	access, ok := m.accessKeys[accessKey]
	m.Unlock()
	if !ok {
		return nil, proto.ErrAccessKeyNotFound
	}
	return access, nil
}

func (m *MemoryStore) ListAccessKeys(ctx context.Context, projectID uint64, active *bool, service *proto.Service) ([]*proto.AccessKey, error) {
	m.Lock()
	defer m.Unlock()
	accessKeys := []*proto.AccessKey{}
	for i, v := range m.accessKeys {
		if v.ProjectID != projectID {
			continue
		}
		if active != nil && *active != v.Active {
			continue
		}
		if service != nil && !v.ValidateService(service) {
			continue
		}
		accessKeys = append(accessKeys, m.accessKeys[i])
	}
	return accessKeys, nil
}

func (m *MemoryStore) GetAccountUsage(ctx context.Context, projectID uint64, service *proto.Service, min, max time.Time) (proto.AccessUsage, error) {
	m.Lock()
	defer m.Unlock()
	usage := proto.AccessUsage{}
	if m.usage.ByProjectID[projectID] != nil {
		usage.Add(*m.usage.ByProjectID[projectID])
	}
	for _, v := range m.accessKeys {
		if v.ProjectID == projectID {
			u, ok := m.usage.ByAccessKey[v.AccessKey]
			if !ok {
				continue
			}
			usage.Add(*u)
		}
	}
	return usage, nil
}

func (m *MemoryStore) GetAccessKeyUsage(ctx context.Context, accessKey string, service *proto.Service, min, max time.Time) (proto.AccessUsage, error) {
	m.Lock()
	defer m.Unlock()
	if _, ok := m.accessKeys[accessKey]; !ok {
		return proto.AccessUsage{}, proto.ErrAccessKeyNotFound
	}
	usage, ok := m.usage.ByAccessKey[accessKey]
	if !ok {
		return proto.AccessUsage{}, nil
	}
	return *usage, nil
}

func (m *MemoryStore) UpdateAccessUsage(ctx context.Context, projectID uint64, accessKey string, service proto.Service, time time.Time, usage proto.AccessUsage) error {
	m.Lock()
	defer m.Unlock()
	if _, ok := m.usage.ByAccessKey[accessKey]; !ok {
		m.usage.ByAccessKey[accessKey] = &usage
		return nil
	}
	m.usage.ByAccessKey[accessKey].Add(usage)
	return nil
}

func (m *MemoryStore) ResetUsage(ctx context.Context, accessKey string, service proto.Service) error {
	m.Lock()
	m.usage.ByAccessKey[accessKey] = &proto.AccessUsage{}
	m.Unlock()
	return nil
}
