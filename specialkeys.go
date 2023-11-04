package quotacontrol

import "sync"

func newSpecialKeys(keys map[string]uint64) *specialKeys {
	return &specialKeys{
		keys: keys,
	}
}

type specialKeys struct {
	sync.RWMutex
	keys map[string]uint64
}

func (s *specialKeys) Get(key string) (uint64, bool) {
	s.RLock()
	defer s.RUnlock()
	val, ok := s.keys[key]
	return val, ok
}

func (s *specialKeys) Set(key string, val uint64) {
	s.Lock()
	defer s.Unlock()
	s.keys[key] = val
}

func (s *specialKeys) Delete(key string) {
	s.Lock()
	defer s.Unlock()
	delete(s.keys, key)
}
