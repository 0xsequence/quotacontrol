//go:generate go run github.com/webrpc/webrpc/cmd/webrpc-gen -schema=quotacontrol.ridl -target=golang -pkg=proto -server -client -out=./quotacontrol.gen.go
//go:generate go run github.com/webrpc/webrpc/cmd/webrpc-gen -schema=quotacontrol.ridl -target=typescript -client -out=./quotacontrol.gen.ts
package proto

import (
	"fmt"
	"slices"
	"time"
)

func Ptr[T any](v T) *T {
	return &v
}

func (t *AccessKey) ValidateOrigin(rawOrigin string) bool {
	if rawOrigin == "" {
		return !t.RequireOrigin
	}
	return t.AllowedOrigins.MatchAny(rawOrigin)
}

func (t *AccessKey) ValidateService(service Service) bool {
	if len(t.AllowedServices) == 0 {
		return true
	}
	for _, s := range t.AllowedServices {
		if service == s {
			return true
		}
	}
	return false
}

func (t *AccessKey) ValidateChains(chainIDs []uint64) error {
	if len(t.ChainIDs) == 0 {
		return nil
	}

	invalid := make([]uint64, 0, len(chainIDs))
	for _, id := range chainIDs {
		if !slices.Contains(t.ChainIDs, id) {
			invalid = append(invalid, id)
		}
	}

	if len(invalid) != 0 {
		return fmt.Errorf("invalid chain IDs: %v", invalid)
	}
	return nil
}

func (l Limit) Validate() error {
	for name, cfg := range l.ServiceLimit {
		svc, ok := ParseService(name)
		if !ok {
			return fmt.Errorf("unknown service %s", name)
		}
		if err := cfg.Validate(); err != nil {
			return fmt.Errorf("service %s: %w", svc.GetName(), err)
		}
	}
	return nil
}

func (l Limit) GetSettings(svc Service) (ServiceLimit, bool) {
	settings, ok := l.GetSettings(svc)
	return settings, ok
}

func (l ServiceLimit) Validate() error {
	if l.RateLimit < 1 {
		return fmt.Errorf("rateLimit must be > 0")
	}
	if l.FreeMax <= 0 {
		return fmt.Errorf("freeMax must be > 0")
	}
	if l.FreeWarn != 0 && l.FreeWarn > l.FreeMax {
		return fmt.Errorf("freeWarn must be >= 0 and <= freeMax")
	}
	if l.OverMax < l.FreeMax {
		return fmt.Errorf("overMax must be >= freeMax")
	}
	if l.OverWarn != 0 && l.OverWarn > l.OverMax {
		return fmt.Errorf("overWarn must be >= 0 and <= overMax")
	}
	return nil
}

// getOverThreshold returns the amount over the threshold
func getOverThreshold(v, total, threshold int64) (int64, bool) {
	if total < threshold {
		return 0, false
	}
	if before := total - v; before >= threshold {
		return 0, false
	}
	return max(0, total-threshold), true
}

func (l *ServiceLimit) GetSpendResult(v, total int64) (int64, *EventType) {
	// valid usage
	if total < l.FreeMax {
		// threshold of included alert
		if _, ok := getOverThreshold(v, total, l.FreeWarn); ok {
			return v, Ptr(EventType_FreeWarn)
		}
		// normal valid usage
		return v, nil
	}

	// overage usage
	if total < l.OverMax {
		// threshold of included limit
		if _, ok := getOverThreshold(v, total, l.FreeMax); ok {
			return v, Ptr(EventType_FreeMax)
		}
		// threshold of overage alert
		if _, ok := getOverThreshold(v, total, l.OverWarn); ok {
			return v, Ptr(EventType_OverWarn)
		}
		// normal overage usage
		return v, nil
	}

	// limited usage
	if over, ok := getOverThreshold(v, total, l.OverMax); ok {
		return v - over, Ptr(EventType_OverMax)
	}
	return 0, nil
}

func (q *AccessQuota) IsActive() bool {
	if q.Limit == nil || q.AccessKey == nil {
		return false
	}
	return q.AccessKey.Active
}

func (q *AccessQuota) IsJWT() bool {
	return q.AccessKey != nil && q.AccessKey.AccessKey == ""
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

func (u *UserPermission) CanAccess(perm UserPermission) bool {
	if u == nil {
		return false
	}
	return *u >= perm
}

func (e WebRPCError) WithMessage(message string) WebRPCError {
	err := e
	if message != "" {
		err.Message = message
	}
	return err
}

func ParseService(v string) (Service, bool) {
	raw, ok := Service_value[v]
	if !ok {
		return 0, false
	}
	return Service(raw), true
}

func (x Service) GetName() string {
	switch x {
	case 0:
		return "node-gateway"
	case 1:
		return "api"
	case 2:
		return "indexer"
	case 3:
		return "relayer"
	case 4:
		return "metadata"
	case 5:
		return "marketplace"
	case 6:
		return "builder"
	case 7:
		return "waas"
	case 8:
		return "trails"
	}
	return ""
}

func (x Service) GetService() Service {
	return x
}
