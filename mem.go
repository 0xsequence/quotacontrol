package quotacontrol

import (
	"context"
	"time"

	"github.com/0xsequence/quotacontrol/proto"
)

// NewMemoryStore returns a new in-memory store.
func NewMemoryStore() *MemoryStore {
	return &MemoryStore{
		limits: map[uint64]*proto.Limit{},
		tokens: map[string]*proto.AccessToken{},
		usage:  map[string]*proto.AccessTokenUsage{},
	}
}

// MemoryStore is an in-memory store, used for testing and prototype.
type MemoryStore struct {
	limits map[uint64]*proto.Limit
	tokens map[string]*proto.AccessToken
	usage  map[string]*proto.AccessTokenUsage
}

func (m MemoryStore) SetAccessLimit(ctx context.Context, projectID uint64, config *proto.Limit) error {
	m.limits[projectID] = config
	return nil
}

func (m MemoryStore) GetAccessLimit(ctx context.Context, projectID uint64) (*proto.Limit, error) {
	limit, ok := m.limits[projectID]
	if !ok {
		return nil, proto.ErrTokenNotFound
	}
	return limit, nil
}

func (m MemoryStore) InsertToken(ctx context.Context, token *proto.AccessToken) error {
	m.tokens[token.TokenKey] = token
	return nil
}

func (m MemoryStore) UpdateToken(ctx context.Context, token *proto.AccessToken) (*proto.AccessToken, error) {
	m.tokens[token.TokenKey] = token
	return token, nil
}

func (m MemoryStore) FindByTokenKey(ctx context.Context, tokenKey string) (*proto.AccessToken, error) {
	token, ok := m.tokens[tokenKey]
	if !ok {
		return nil, proto.ErrTokenNotFound
	}
	return token, nil
}

func (m MemoryStore) ListByProjectID(ctx context.Context, projectID uint64, active *bool) ([]*proto.AccessToken, error) {
	tokens := []*proto.AccessToken{}
	for i, v := range m.tokens {
		if v.ProjectID == projectID {
			tokens = append(tokens, m.tokens[i])
		}
	}
	return tokens, nil
}

func (m MemoryStore) GetAccountTotalUsage(ctx context.Context, projectID uint64, service *proto.Service, min, max time.Time) (proto.AccessTokenUsage, error) {
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
