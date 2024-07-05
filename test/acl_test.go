package test

import (
	"testing"

	"github.com/0xsequence/quotacontrol/middleware"
	"github.com/0xsequence/quotacontrol/proto"
	"github.com/stretchr/testify/assert"
)

func TestVerifyACL(t *testing.T) {
	type Service interface {
		Method1() error
		Method2() error
	}

	acl := middleware.ACL{
		"Service": {
			"Method1": proto.SessionType_Account,
		},
	}

	err := VerifyACL[Service](acl)
	assert.Error(t, err)

	acl = middleware.ACL{
		"Service": {
			"Method1": proto.SessionType_Account,
			"Method2": proto.SessionType_Account,
		},
	}

	err = VerifyACL[Service](acl)
	assert.NoError(t, err)
}
