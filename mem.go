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
		limits:     map[uint64]proto.Limit{},
		accessKeys: map[string]proto.AccessKey{},
		usage: usageRecord{
			ByProjectID: map[uint64]*proto.AccessUsage{},
			ByAccessKey: map[string]*proto.AccessUsage{},
		},
		users:       map[string]bool{},
		permissions: map[uint64]map[string]userPermission{},
	}
}

type userPermission struct {
	Permission proto.UserPermission
	Access     proto.ResourceAccess
}

// MemoryStore is an in-memory store, used for testing and prototype.
type MemoryStore struct {
	sync.Mutex
	limits      map[uint64]proto.Limit
	cycles      map[uint64]proto.Cycle
	accessKeys  map[string]proto.AccessKey
	usage       usageRecord
	users       map[string]bool
	permissions map[uint64]map[string]userPermission
}

func (m *MemoryStore) SetAccessLimit(ctx context.Context, projectID uint64, config *proto.Limit) error {
	m.Lock()
	m.limits[projectID] = *config
	m.Unlock()
	return nil
}

func (m *MemoryStore) GetAccessLimit(ctx context.Context, projectID uint64, cycle *proto.Cycle) (*proto.Limit, error) {
	m.Lock()
	limit, ok := m.limits[projectID]
	m.Unlock()
	if !ok {
		return nil, proto.ErrAccessKeyNotFound
	}
	return &limit, nil
}

func (m *MemoryStore) SetAccessCycle(ctx context.Context, projectID uint64, cycle *proto.Cycle) error {
	m.Lock()
	if cycle != nil {
		m.cycles[projectID] = *cycle
	} else {
		delete(m.cycles, projectID)
	}
	m.Unlock()
	return nil
}

func (m *MemoryStore) GetAccessCycle(ctx context.Context, projectID uint64, now time.Time) (*proto.Cycle, error) {
	m.Lock()
	cycle := m.cycles[projectID]
	m.Unlock()
	if cycle.Start.IsZero() && cycle.End.IsZero() {
		return DefaultCycleStore{}.GetAccessCycle(ctx, projectID, now)
	}
	return &cycle, nil
}

func (m *MemoryStore) InsertAccessKey(ctx context.Context, access *proto.AccessKey) error {
	m.Lock()
	m.accessKeys[access.AccessKey] = *access
	m.Unlock()
	return nil
}

func (m *MemoryStore) UpdateAccessKey(ctx context.Context, access *proto.AccessKey) (*proto.AccessKey, error) {
	m.Lock()
	m.accessKeys[access.AccessKey] = *access
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
	return &access, nil
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
		if service != nil && !v.ValidateService(*service) {
			continue
		}
		accessKeys = append(accessKeys, proto.Ptr(m.accessKeys[i]))
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

func (m *MemoryStore) GetAccessKeyUsage(ctx context.Context, projectID uint64, accessKey string, service *proto.Service, min, max time.Time) (proto.AccessUsage, error) {
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

func (m *MemoryStore) ResetUsage(ctx context.Context, accessKey string) error {
	m.Lock()
	m.usage.ByAccessKey[accessKey] = &proto.AccessUsage{}
	m.Unlock()
	return nil
}

func (m *MemoryStore) AddUser(ctx context.Context, userID string, admin bool) error {
	m.Lock()
	m.users[userID] = admin
	m.Unlock()
	return nil
}

func (m *MemoryStore) GetUser(ctx context.Context, userID string) (any, bool, error) {
	m.Lock()
	v, ok := m.users[userID]
	m.Unlock()
	if !ok {
		return nil, false, nil
	}
	return userID, v, nil
}

func (m *MemoryStore) GetUserPermission(ctx context.Context, projectID uint64, userID string) (proto.UserPermission, *proto.ResourceAccess, error) {
	m.Lock()
	defer m.Unlock()
	users, ok := m.permissions[projectID]
	if !ok {
		return proto.UserPermission_UNAUTHORIZED, nil, nil
	}
	p, ok := users[userID]
	if !ok {
		return proto.UserPermission_UNAUTHORIZED, nil, nil
	}
	return p.Permission, &p.Access, nil
}

func (m *MemoryStore) SetUserPermission(ctx context.Context, projectID uint64, userID string, permission proto.UserPermission, access proto.ResourceAccess) error {
	m.Lock()
	defer m.Unlock()
	if m.permissions[projectID] == nil {
		m.permissions[projectID] = make(map[string]userPermission)
	}
	m.permissions[projectID][userID] = userPermission{Permission: permission, Access: access}
	return nil
}
