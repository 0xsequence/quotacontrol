package quotacontrol

import (
	"fmt"
	"time"

	"github.com/0xsequence/quotacontrol/proto"
)

const Version = "v2"

// KeyUsage is a cache key for usage amounts.
// It does not include version because usage is just a number, and it's safe to share across versions.
type KeyUsage struct {
	ProjectID uint64
	Service   proto.Service
	Start     time.Time
	End       time.Time
}

func newKeyUsage(projectID uint64, service proto.Service, cycle *proto.Cycle, now time.Time) KeyUsage {
	return KeyUsage{
		ProjectID: projectID,
		Service:   service,
		Start:     cycle.GetStart(now),
		End:       cycle.GetEnd(now),
	}
}

func (k KeyUsage) String() string {
	return fmt.Sprintf("usage:%s:%d:%s-%s", k.Service.String(), k.ProjectID, k.Start.Format("2006-01-02"), k.End.Format("2006-01-02"))
}

// KeyAccessKey is a cache key for AccessQuota indexed by access key string.
// It includes version to avoid conflicts when the structure changes.
type KeyAccessKey struct {
	AccessKey string
}

func (k KeyAccessKey) String() string {
	return fmt.Sprintf("quota:%s:%s", Version, k.AccessKey)
}

// KeyProject is a cache key for AccessQuota indexed by project ID.
// It includes version to avoid conflicts when the structure changes.
type KeyProject struct {
	ProjectID uint64
}

func (k KeyProject) String() string {
	return fmt.Sprintf("project:%s:%d", Version, k.ProjectID)
}

// KeyPermission is a cache key for user permission indexed by project ID and user ID.
// It includes version to avoid conflicts when the structure changes.
type KeyPermission struct {
	ProjectID uint64
	UserID    string
}

func (k KeyPermission) String() string {
	return fmt.Sprintf("perm:%s:%d:%s", Version, k.ProjectID, k.UserID)
}

type UserPermission struct {
	UserPermission proto.UserPermission  `json:"userPerm"`
	ResourceAccess *proto.ResourceAccess `json:"resourceAccess"`
}
