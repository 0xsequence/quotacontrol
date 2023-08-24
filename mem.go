package quotacontrol

import (
	"context"
	"time"

	"github.com/0xsequence/quotacontrol/proto"
)

// NewMemoryStore returns a new in-memory store.
func NewMemoryStore() *MemoryStore {
	return &MemoryStore{
		limits: map[uint64]map[proto.Service]*proto.ServiceLimit{},
		tokens: map[string]*proto.AccessToken{},
		usage:  map[string]*proto.AccessTokenUsage{},
	}
}

// MemoryStore is an in-memory store, used for testing and prototype.
type MemoryStore struct {
	limits map[uint64]map[proto.Service]*proto.ServiceLimit
	tokens map[string]*proto.AccessToken
	usage  map[string]*proto.AccessTokenUsage
}

func (m MemoryStore) InsertAccessLimit(ctx context.Context, projectID uint64, limit *proto.ServiceLimit) error {
	if _, ok := m.limits[projectID]; !ok {
		m.limits[projectID] = map[proto.Service]*proto.ServiceLimit{}
	}
	m.limits[projectID][*limit.Service] = limit
	return nil
}

func (m MemoryStore) GetAccessLimit(ctx context.Context, projectID uint64) ([]*proto.ServiceLimit, error) {
	limit, ok := m.limits[projectID]
	if !ok {
		return nil, proto.ErrTokenNotFound
	}
	var result []*proto.ServiceLimit
	for _, v := range limit {
		result = append(result, v)
	}
	return result, nil
}

func (m MemoryStore) InsertToken(ctx context.Context, token *proto.AccessToken) error {
	m.tokens[token.TokenKey] = token
	return nil
}

func (m MemoryStore) FindByTokenKey(ctx context.Context, tokenKey string) (*proto.AccessToken, error) {
	token, ok := m.tokens[tokenKey]
	if !ok {
		return nil, proto.ErrTokenNotFound
	}
	return token, nil
}

func (m MemoryStore) GetAccountTotalUsage(ctx context.Context, projectID uint64, service proto.Service, min, max time.Time) (proto.AccessTokenUsage, error) {
	usage := proto.AccessTokenUsage{}
	for _, v := range m.tokens {
		if v.ProjectID == projectID {
			u, ok := m.usage[v.TokenKey]
			if !ok {
				continue
			}
			usage.Add(*u)
		}
	}
	return usage, nil
}

func (m MemoryStore) UpdateTokenUsage(ctx context.Context, tokenKey string, service proto.Service, time time.Time, usage proto.AccessTokenUsage) error {
	if _, ok := m.usage[tokenKey]; !ok {
		m.usage[tokenKey] = &usage
		return nil
	}
	m.usage[tokenKey].Add(usage)
	return nil
}

func (m MemoryStore) ResetUsage(ctx context.Context, tokenKey string, service proto.Service) error {
	m.usage[tokenKey] = &proto.AccessTokenUsage{}
	return nil
}
