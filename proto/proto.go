//go:generate go run github.com/webrpc/webrpc/cmd/webrpc-gen -schema=proto.ridl -target=golang@v0.12.1 -pkg=proto -server -client -out=./proto.gen.go
//go:generate go run github.com/webrpc/webrpc/cmd/webrpc-gen -schema=proto.ridl -target=typescript@v0.12.0 -client -out=./clients/builder.gen.ts

package proto

import (
	"fmt"
	"time"
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

func (s *Service) GetQuotaKey(projectID uint64, now time.Time) string {
	return fmt.Sprintf("%v_%v_%s", s, projectID, now.Format("2006-01"))
}

func (s *ServiceLimit) Validate() error {
	if s.Service == nil {
		return fmt.Errorf("service must be set")
	}
	if s.ComputeRateLimit <= 0 {
		return fmt.Errorf("computeRateLimit must be >= 0")
	}
	if s.ComputeMonthlyQuota <= 0 {
		return fmt.Errorf("computeMonthlyQuota must be >= 0")
	}
	if s.ComputeMonthlyHardQuota <= 0 {
		return fmt.Errorf("computeMonthlyHardQuota must be >= 0")
	}
	if s.ComputeMonthlyQuota > s.ComputeMonthlyHardQuota {
		return fmt.Errorf("computeMonthlyQuota must be <= computeMonthlyHardQuota")
	}
	return nil
}
