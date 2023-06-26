package proto

import (
	"fmt"
	"time"
)

//go:generate go run github.com/webrpc/webrpc/cmd/webrpc-gen -schema=proto.ridl -target=golang@v0.10.0 -pkg=. -server -client -out=./proto.gen.go

func Ptr[T any](v T) *T {
	return &v
}

func (u *AccessTokenUsage) Add(usage AccessTokenUsage) {
	u.LimitedCompute += usage.LimitedCompute
	u.ValidCompute += usage.ValidCompute
}

func (t *AccessToken) ValidateOrigin(origin string) bool {
	if len(t.AllowedOrigins) == 0 {
		return true
	}
	for _, o := range t.AllowedOrigins {
		if origin == o {
			return true
		}
	}
	return false
}

func (s *Service) GetQuotaKey(dappId uint64, now time.Time) string {
	return fmt.Sprintf("%v_%v_%s", s, dappId, now.Format("2006-01"))
}
