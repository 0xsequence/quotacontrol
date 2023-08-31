//go:generate go run github.com/webrpc/webrpc/cmd/webrpc-gen -schema=proto.ridl -target=golang@v0.12.1 -pkg=proto -server -client -out=./proto.gen.go
//go:generate go run github.com/webrpc/webrpc/cmd/webrpc-gen -schema=proto.ridl -target=typescript@v0.12.0 -client -out=./clients/builder.gen.ts

package proto

import (
	"fmt"
)

func Ptr[T any](v T) *T {
	return &v
}

func (u *AccessTokenUsage) Add(usage AccessTokenUsage) {
	u.LimitedCompute += usage.LimitedCompute
	u.ValidCompute += usage.ValidCompute
	u.OverCompute += usage.OverCompute
}

func (u *AccessTokenUsage) GetTotalUsage() int64 {
	return u.ValidCompute + u.OverCompute
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
func (t *AccessToken) ValidateService(service *Service) bool {
	if len(t.AllowedServices) == 0 {
		return true
	}
	for _, s := range t.AllowedServices {
		if *service == *s {
			return true
		}
	}
	return false
}

func (l *Limit) Validate() error {
	if l.RateLimit < 1 {
		return fmt.Errorf("rateLimit must be > 0")
	}
	if l.FreeCU < 0 {
		return fmt.Errorf("freeCU must be >= 0")
	}
	if l.SoftQuota <= 0 || l.SoftQuota < l.FreeCU {
		return fmt.Errorf("softQuota must be >= 0 and >= freeCU")
	}
	if l.HardQuota <= 0 || l.HardQuota < l.SoftQuota {
		return fmt.Errorf("hardQuota must be >= 0 and >= softQuota")
	}
	return nil
}
