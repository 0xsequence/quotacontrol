package proto

import (
	"fmt"
	"strings"
	"time"
)

//go:generate go run github.com/webrpc/webrpc/cmd/webrpc-gen -schema=proto.ridl -target=golang@v0.13.0 -pkg=proto -server -client -out=./proto.gen.go
//go:generate go run github.com/webrpc/webrpc/cmd/webrpc-gen -schema=proto.ridl -target=typescript@v0.12.0 -client -out=./clients/builder.gen.ts

func Ptr[T any](v T) *T {
	return &v
}

func (u *AccessUsage) Add(usage AccessUsage) {
	u.LimitedCompute += usage.LimitedCompute
	u.ValidCompute += usage.ValidCompute
	u.OverCompute += usage.OverCompute
}

func (u *AccessUsage) GetTotalUsage() int64 {
	return u.ValidCompute + u.OverCompute
}

func matchDomain(domain, pattern string) bool {
	if pattern == "*" {
		return true
	}

	prefix, suffix, found := strings.Cut(pattern, "*")
	if found {
		return len(domain) >= len(prefix+suffix) && strings.HasPrefix(domain, prefix) && strings.HasSuffix(domain, suffix)
	}

	return domain == pattern
}

func (t *AccessKey) ValidateOrigin(rawOrigin string) bool {
	if len(t.AllowedOrigins) == 0 {
		return true
	}

	origin := strings.ToLower(rawOrigin)
	for _, o := range t.AllowedOrigins {
		if matchDomain(origin, o) {
			return true
		}
	}
	return false
}

func (t *AccessKey) ValidateService(service *Service) bool {
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

func (cfg *Limit) GetSpendResult(cu, total int64) (u AccessUsage, e *EventType) {
	switch {
	case total < cfg.FreeCU:
		u.ValidCompute = cu
	case total < cfg.SoftQuota:
		if before := total - cu; before < cfg.FreeCU {
			u.ValidCompute += cfg.FreeCU - before
			e = Ptr(EventType_FreeCU)
		}
		u.OverCompute += cu - u.ValidCompute
	case total >= cfg.HardQuota:
		if before := total - cu; before < cfg.HardQuota {
			u.OverCompute += cfg.HardQuota - before
			e = Ptr(EventType_HardQuota)
		}
		u.LimitedCompute += cu - u.OverCompute
	default:
		if total-cu < cfg.SoftQuota && total >= cfg.SoftQuota {
			e = Ptr(EventType_SoftQuota)
		}
		u.OverCompute = cu
	}
	return
}

func (q *AccessQuota) IsActive() bool {
	if q.Limit == nil || q.AccessKey == nil {
		return false
	}
	return q.AccessKey.Active
}

func (q *AccessQuota) IsDefault() bool {
	if q.Limit == nil || q.AccessKey == nil {
		return false
	}
	return q.AccessKey.Default
}

func (q *AccessQuota) GetProjectID() uint64 {
	if q.AccessKey == nil {
		return 0
	} else {
		return q.AccessKey.ProjectID
	}
}

func (c *Cycle) GetStart(now time.Time) time.Time {
	if c != nil && !c.Start.IsZero() {
		return c.Start
	}
	return time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())
}

func (c *Cycle) GetEnd(now time.Time) time.Time {
	if c != nil && !c.End.IsZero() {
		return c.End
	}
	return c.GetStart(now).AddDate(0, 1, -1)
}

func (c *Cycle) GetDuration(now time.Time) time.Duration {
	return c.GetEnd(now).Sub(c.GetStart(now))
}

func (c *Cycle) Advance(now time.Time) {
	for c.End.Before(now) {
		c.Start = c.Start.AddDate(0, 1, 0)
		c.End = c.End.AddDate(0, 1, 0)
	}

}
