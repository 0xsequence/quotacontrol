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

	acl := middleware.ServiceConfig[middleware.ACL]{
		"Service": {
			"Method1": middleware.NewACL(proto.SessionType_Wallet.AndUp()...),
		},
	}

	err := VerifyACL[Service](acl)
	assert.Error(t, err)

	acl = middleware.ServiceConfig[middleware.ACL]{
		"Service": {
			"Method1": middleware.NewACL(proto.SessionType_Wallet.AndUp()...),
			"Method2": middleware.NewACL(proto.SessionType_Wallet.AndUp()...),
		},
	}

	err = VerifyACL[Service](acl)
	assert.NoError(t, err)
}
