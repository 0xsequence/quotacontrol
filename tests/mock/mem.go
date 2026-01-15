package mock

import (
	"context"
	"sync"
	"time"

	"github.com/0xsequence/authcontrol"
	"github.com/0xsequence/quotacontrol/internal/usage"
	"github.com/0xsequence/quotacontrol/proto"
)

// NewMemoryStore returns a new in-memory store.
func NewMemoryStore() *MemoryStore {
	ms := MemoryStore{
		infos:       map[uint64]proto.ProjectInfo{},
		limits:      map[uint64]map[string]proto.Limit{},
		accessKeys:  map[string]proto.AccessKey{},
		usage:       map[proto.Service]usage.Record{},
		users:       map[string]bool{},
		projects:    map[uint64]*authcontrol.Auth{},
		permissions: map[uint64]map[string]userPermission{},
	}
	for i := range proto.Service_name {
		svc := proto.Service(i)
		ms.usage[svc] = usage.NewRecord()
	}
	return &ms
}

type userPermission struct {
	Permission proto.UserPermission
	Access     proto.ResourceAccess
}

// MemoryStore is an in-memory store, used for testing and prototype.
type MemoryStore struct {
	sync.Mutex
	infos       map[uint64]proto.ProjectInfo
	limits      map[uint64]map[string]proto.Limit
	accessKeys  map[string]proto.AccessKey
	usage       map[proto.Service]usage.Record
	users       map[string]bool
	projects    map[uint64]*authcontrol.Auth
	permissions map[uint64]map[string]userPermission
}

func (m *MemoryStore) SetProjectInfo(ctx context.Context, info proto.ProjectInfo) error {
	m.Lock()
	m.infos[info.ProjectID] = info
	m.Unlock()
	return nil
}

func (m *MemoryStore) GetProjectInfo(ctx context.Context, projectID uint64) (*proto.ProjectInfo, error) {
	m.Lock()
	info, ok := m.infos[projectID]
	m.Unlock()
	if !ok {
		return nil, proto.ErrProjectNotFound
	}
	return &info, nil
}

func (m *MemoryStore) SetLimit(ctx context.Context, projectID uint64, service proto.Service, limit proto.Limit) error {
	m.Lock()
	if _, ok := m.infos[projectID]; !ok {
		m.infos[projectID] = proto.ProjectInfo{ProjectID: projectID}
	}
	if m.limits[projectID] == nil {
		m.limits[projectID] = make(map[string]proto.Limit)
	}
	m.limits[projectID][service.String()] = limit
	m.Unlock()
	return nil
}

func (m *MemoryStore) GetLimit(ctx context.Context, projectID uint64, svc proto.Service) (*proto.Limit, error) {
	m.Lock()
	limits, ok := m.limits[projectID]
	m.Unlock()
	if !ok {
		return nil, proto.ErrAccessKeyNotFound
	}
	limit, ok := limits[svc.String()]
	if !ok {
		return nil, proto.ErrInvalidService
	}
	return &limit, nil
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

func (m *MemoryStore) GetAccountUsage(ctx context.Context, projectID uint64, service *proto.Service, min, max time.Time) (int64, error) {
	m.Lock()
	defer m.Unlock()
	var usage int64
	if service != nil {
		usage += m.usage[*service].ByProjectID[projectID]
		for _, v := range m.accessKeys {
			if v.ProjectID == projectID {
				usage += m.usage[*service].ByAccessKey[v.AccessKey]
			}
		}
		return usage, nil
	}
	for _, v := range m.usage {
		usage += v.ByProjectID[projectID]
		for _, u := range v.ByAccessKey {
			usage += u
		}
	}
	return usage, nil
}

func (m *MemoryStore) GetAccessKeyUsage(ctx context.Context, projectID uint64, accessKey string, service *proto.Service, min, max time.Time) (int64, error) {
	m.Lock()
	defer m.Unlock()
	if _, ok := m.accessKeys[accessKey]; !ok {
		return 0, proto.ErrAccessKeyNotFound
	}
	if service != nil {
		usage, ok := m.usage[*service].ByAccessKey[accessKey]
		if !ok {
			return 0, nil
		}
		return usage, nil
	}
	var usage int64
	for _, v := range m.usage {
		if u, ok := v.ByAccessKey[accessKey]; ok {
			usage += u
		}
	}
	return usage, nil
}

func (m *MemoryStore) InsertAccessUsage(ctx context.Context, projectID uint64, accessKey string, service proto.Service, time time.Time, usage int64) error {
	m.Lock()
	defer m.Unlock()
	if _, ok := m.usage[service].ByAccessKey[accessKey]; !ok {
		m.usage[service].ByAccessKey[accessKey] = usage
		return nil
	}
	m.usage[service].ByAccessKey[accessKey] += usage
	return nil
}

func (m *MemoryStore) ResetUsage(ctx context.Context, accessKey string, service *proto.Service) error {
	m.Lock()
	m.usage[*service].ByAccessKey[accessKey] = 0
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
	return struct{}{}, v, nil
}

func (m *MemoryStore) AddProject(ctx context.Context, projectID uint64, auth *authcontrol.Auth) error {
	m.Lock()
	m.projects[projectID] = auth
	m.Unlock()
	return nil
}

func (m *MemoryStore) GetProject(ctx context.Context, projectID uint64) (any, *authcontrol.Auth, error) {
	m.Lock()
	auth, ok := m.projects[projectID]
	m.Unlock()
	if !ok {
		return nil, nil, nil
	}
	return struct{}{}, auth, nil
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
