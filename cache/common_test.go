package cache_test

import "fmt"

type Key string

func (k Key) String() string {
	return string(k)
}

type UserKey uint64

func (k UserKey) String() string {
	return "user:" + fmt.Sprintf("%d", k)
}

type UsageKey uint64

func (k UsageKey) String() string {
	return fmt.Sprintf("usage:%d", k)
}
