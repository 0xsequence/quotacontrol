package proto_test

import (
	"testing"

	"github.com/0xsequence/quotacontrol/proto"
	"github.com/stretchr/testify/assert"
)

func TestAccessTokenValidateOrigin(t *testing.T) {
	t.Run("no allowed origins", func(t *testing.T) {
		tk := &proto.AccessToken{}
		assert.True(t, tk.ValidateOrigin("http://localhost:8080"))
	})

	t.Run("allowed origins", func(t *testing.T) {
		tk := &proto.AccessToken{
			AllowedOrigins: []string{"http://localhost:8080", "http://localhost:8081"},
		}
		assert.True(t, tk.ValidateOrigin("http://localhost:8080"))
		assert.True(t, tk.ValidateOrigin("http://localhost:8081"))
		assert.False(t, tk.ValidateOrigin("http://localhost:8082"))
	})
}

func TestGetSpendResult(t *testing.T) {
	const (
		_CU   = 5
		_Free = _CU * 2
		_Soft = _CU * 4
		_Hard = _CU * 8
	)

	type TestCase struct {
		Name  string
		Total int64

		Usage proto.AccessTokenUsage
		Event *proto.EventType
	}

	for _, tc := range []TestCase{
		{
			Name:  "WithinFreeCU",
			Total: _Free - 1,
			Usage: proto.AccessTokenUsage{ValidCompute: _CU},
			Event: nil,
		}, {
			Name:  "WithinFreeCUExact",
			Total: _Free,
			Usage: proto.AccessTokenUsage{ValidCompute: _CU},
			Event: proto.EventType_FreeCU.Ptr(),
		}, {
			Name:  "OverFreeCU",
			Total: _Free + 2,
			Usage: proto.AccessTokenUsage{ValidCompute: _CU - 2, OverCompute: 2},
			Event: proto.EventType_FreeCU.Ptr(),
		}, {
			Name:  "OverFreeCUExact",
			Total: _Free + _CU,
			Usage: proto.AccessTokenUsage{OverCompute: _CU},
			Event: nil,
		}, {
			Name:  "WithinSoft",
			Total: _Soft - 1,
			Usage: proto.AccessTokenUsage{OverCompute: _CU},
			Event: nil,
		}, {
			Name:  "WithinSoftExact",
			Total: _Soft,
			Usage: proto.AccessTokenUsage{OverCompute: _CU},
			Event: proto.EventType_SoftQuota.Ptr(),
		}, {
			Name:  "OverSoft",
			Total: _Soft + 2,
			Usage: proto.AccessTokenUsage{OverCompute: _CU},
			Event: proto.EventType_SoftQuota.Ptr(),
		}, {
			Name:  "OverSoftExact",
			Total: _Soft + _CU,
			Usage: proto.AccessTokenUsage{OverCompute: _CU},
			Event: nil,
		}, {
			Name:  "WithinHard",
			Total: _Hard - 1,
			Usage: proto.AccessTokenUsage{OverCompute: _CU},
			Event: nil,
		}, {
			Name:  "OverHardExact",
			Total: _Hard,
			Usage: proto.AccessTokenUsage{OverCompute: _CU},
			Event: proto.EventType_HardQuota.Ptr(),
		}, {
			Name:  "OverHard",
			Total: _Hard + 2,
			Usage: proto.AccessTokenUsage{OverCompute: _CU - 2, LimitedCompute: 2},
			Event: proto.EventType_HardQuota.Ptr(),
		},
	} {
		t.Run(tc.Name, func(t *testing.T) {
			limit := proto.Limit{
				FreeCU:    _Free,
				SoftQuota: _Soft,
				HardQuota: _Hard,
			}
			u, evt := limit.GetSpendResult(_CU, tc.Total)
			assert.Equal(t, tc.Usage, u)
			assert.Equal(t, tc.Event, evt)
		})
	}

}
