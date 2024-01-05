package proto

import (
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
	"time"
)

//go:generate go run github.com/webrpc/webrpc/cmd/webrpc-gen -schema=proto.ridl -target=golang@v0.13.7 -pkg=proto -server -client -out=./proto.gen.go
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
	if strings.HasPrefix(pattern, "*.") {
		return strings.HasSuffix(domain, pattern[1:])
	}
	return domain == pattern
}

func (t *AccessKey) ValidateOrigin(rawOrigin string) bool {
	if len(t.AllowedOrigins) == 0 {
		return true
	}
	origin, err := url.Parse(rawOrigin)
	if err != nil {
		return false
	}
	for _, o := range t.AllowedOrigins {
		if matchDomain(origin.Host, o) {
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

func (l *Limit) MarshalJSON() ([]byte, error) {
	type Alias Limit
	var v = struct {
		*Alias
		CreditsIncluded int64 `json:"freeCU"`
		SoftQuota       int64 `json:"softQuota"`
		HardQuota       int64 `json:"hardQuota"`
	}{
		Alias:           (*Alias)(l),
		CreditsIncluded: l.FreeWarn,
		SoftQuota:       l.OverWarn,
		HardQuota:       l.OverMax,
	}
	return json.Marshal(v)
}

func (l Limit) Validate() error {
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

func (l *Limit) GetSpendResult(v, total int64) (AccessUsage, *EventType) {
	// valid usage
	if total < l.FreeMax {
		// threshold of included alert
		if _, ok := getOverThreshold(v, total, l.FreeWarn); ok {
			return AccessUsage{ValidCompute: v}, Ptr(EventType_FreeWarn)
		}
		// normal valid usage
		return AccessUsage{ValidCompute: v}, nil
	}

	// overage usage
	if total < l.OverMax {
		// threshold of included limit
		if over, ok := getOverThreshold(v, total, l.FreeMax); ok {
			return AccessUsage{ValidCompute: v - over, OverCompute: over}, Ptr(EventType_FreeMax)
		}
		// threshold of overage alert
		if _, ok := getOverThreshold(v, total, l.OverWarn); ok {
			return AccessUsage{OverCompute: v}, Ptr(EventType_OverWarn)
		}
		// normal overage usage
		return AccessUsage{OverCompute: v}, nil
	}

	// limited usage
	if over, ok := getOverThreshold(v, total, l.OverMax); ok {
		return AccessUsage{LimitedCompute: over, OverCompute: v - over}, Ptr(EventType_OverMax)
	}
	return AccessUsage{LimitedCompute: v}, nil
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
