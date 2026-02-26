//go:generate go run github.com/webrpc/webrpc/cmd/webrpc-gen -schema=quotacontrol.ridl -target=golang -pkg=proto -server -client -out=./quotacontrol.gen.go
//go:generate go run github.com/webrpc/webrpc/cmd/webrpc-gen -schema=quotacontrol.ridl -target=typescript -client -out=./quotacontrol.gen.ts
package proto

import (
	"cmp"
	"fmt"
	"slices"
	"time"
)

// Ptr is an utility function to return a pointer to the value
func Ptr[T any](v T) *T {
	return &v
}

// ValidateOrigin checks if the given origin is allowed by the access key.
func (a *AccessKey) ValidateOrigin(rawOrigin string) bool {
	if rawOrigin == "" {
		return !a.RequireOrigin
	}
	return a.AllowedOrigins.MatchAny(rawOrigin)
}

// ValidateService checks if the given service is allowed by the access key.
func (a *AccessKey) ValidateService(service Service) bool {
	if len(a.AllowedServices) == 0 {
		return true
	}
	for _, s := range a.AllowedServices {
		if service == s {
			return true
		}
	}
	return false
}

// ValidateChains checks if the given chain IDs are allowed by the project.
func (i *ProjectInfo) ValidateChains(chainIDs []uint64) error {
	if len(i.ChainIDs) == 0 {
		return nil
	}

	invalid := make([]uint64, 0, len(chainIDs))
	for _, id := range chainIDs {
		if !slices.Contains(i.ChainIDs, id) {
			invalid = append(invalid, id)
		}
	}

	if len(invalid) != 0 {
		return fmt.Errorf("invalid chain IDs: %v", invalid)
	}
	return nil
}

// Validate checks if the limit configuration is valid.
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

// GetSettings returns the service limit settings for the given service.
func (l Limit) GetSettings(svc Service) (ServiceLimit, bool) {
	settings, ok := l.ServiceLimit[svc.String()]
	return settings, ok
}

// SetSetting sets the service limit settings for the given service.
func (l *Limit) SetSetting(svc Service, limits ServiceLimit) {
	if l.ServiceLimit == nil {
		l.ServiceLimit = make(map[string]ServiceLimit)
	}
	l.ServiceLimit[svc.String()] = limits
}

// Validate checks if the service limit configuration is valid.
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

// GetSpendResult calculates the spend result and event type based on the service limit and usage
func (l *ServiceLimit) GetSpendResult(spent, total int64) (int64, *EventType) {
	// valid usage
	if total < l.FreeMax {
		// threshold of included alert
		if _, ok := getOverThreshold(spent, total, l.FreeWarn); ok {
			return spent, Ptr(EventType_FreeWarn)
		}
		// normal valid usage
		return spent, nil
	}

	// overage usage
	if total < l.OverMax {
		// threshold of included limit
		if _, ok := getOverThreshold(spent, total, l.FreeMax); ok {
			return spent, Ptr(EventType_FreeMax)
		}
		// threshold of overage alert
		if _, ok := getOverThreshold(spent, total, l.OverWarn); ok {
			return spent, Ptr(EventType_OverWarn)
		}
		// normal overage usage
		return spent, nil
	}

	// limited usage
	if over, ok := getOverThreshold(spent, total, l.OverMax); ok {
		return spent - over, Ptr(EventType_OverMax)
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

func (c *Cycle) SetInterval(from, to *time.Time, now time.Time) {
	from = cmp.Or(from, &time.Time{})
	to = cmp.Or(to, &time.Time{})

	if !from.IsZero() && !to.IsZero() {
		return
	}

	duration := c.GetEnd(now).Sub(c.GetStart(now))
	switch {
	case !to.IsZero():
		*from = to.Add(-duration)
	case !from.IsZero():
		*to = from.Add(duration)
	default:
		*from = c.Start
		*to = c.End
	}
}

func (u *UserPermission) CanAccess(perm UserPermission) bool {
	if u == nil {
		return false
	}
	return *u >= perm
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
