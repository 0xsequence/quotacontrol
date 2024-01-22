package proto_test

import (
	"encoding/json"
	"math"
	"testing"

	"github.com/0xsequence/quotacontrol/proto"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAccessKeyValidateOrigin(t *testing.T) {
	t.Run("no allowed origins", func(t *testing.T) {
		tk := &proto.AccessKey{}
		assert.True(t, tk.ValidateOrigin("http://localhost:8080"))
	})

	t.Run("allowed origins", func(t *testing.T) {
		tk := &proto.AccessKey{
			AllowedOrigins: []string{"localhost:8080", "localhost:8081"},
		}
		assert.True(t, tk.ValidateOrigin("http://localhost:8080"))
		assert.True(t, tk.ValidateOrigin("http://localhost:8081"))
		assert.False(t, tk.ValidateOrigin("http://localhost:8082"))
	})

	t.Run("match http scheme", func(t *testing.T) {
		tk := &proto.AccessKey{
			AllowedOrigins: []string{"https://localhost:8080", "https://*.sequence.xyz"},
		}

		assert.True(t, tk.ValidateOrigin("https://local.sequence.xyz"))
		assert.True(t, tk.ValidateOrigin("https://localhost:8080"))
		assert.False(t, tk.ValidateOrigin("http://localhost:8080"))
	})

	t.Run("wildcards", func(t *testing.T) {
		tk := &proto.AccessKey{
			AllowedOrigins: []string{"*.sequence.xyz"},
		}
		assert.False(t, tk.ValidateOrigin("http://sequence.xyz"))
		assert.True(t, tk.ValidateOrigin("http://docs.sequence.xyz"))
		assert.True(t, tk.ValidateOrigin("http://test.sequence.xyz"))
		assert.True(t, tk.ValidateOrigin("http://dev.test.sequence.xyz"))
	})
}

func TestGetSpendResult(t *testing.T) {
	const (
		_CU = 5
	)
	limit := proto.Limit{
		FreeWarn: 10,
		FreeMax:  20,
		OverWarn: 30,
		OverMax:  40,
	}

	type TestCase struct {
		Name  string
		Total int64

		Usage proto.AccessUsage
		Event *proto.EventType
	}

	for _, tc := range []TestCase{
		// Include Alert
		{
			Name:  "Within_IncludedAlert",
			Total: limit.FreeWarn - 1,
			Usage: proto.AccessUsage{ValidCompute: _CU},
			Event: nil,
		}, {
			Name:  "Within_IncludedAlert_Exact",
			Total: limit.FreeWarn,
			Usage: proto.AccessUsage{ValidCompute: _CU},
			Event: proto.Ptr(proto.EventType_FreeWarn),
		}, {
			Name:  "Above_IncludedAlert",
			Total: limit.FreeWarn + 1,
			Usage: proto.AccessUsage{ValidCompute: _CU},
			Event: proto.Ptr(proto.EventType_FreeWarn),
		}, {
			Name:  "Above_IncludedAlert_Exact",
			Total: limit.FreeWarn + _CU,
			Usage: proto.AccessUsage{ValidCompute: _CU},
			Event: nil,
		},
		// Include Limit
		{
			Name:  "Within_IncludedLimit",
			Total: limit.FreeWarn - 1,
			Usage: proto.AccessUsage{ValidCompute: _CU},
			Event: nil,
		}, {
			Name:  "Within_IncludedLimit_Exact",
			Total: limit.FreeMax,
			Usage: proto.AccessUsage{ValidCompute: _CU},
			Event: proto.Ptr(proto.EventType_FreeMax),
		}, {
			Name:  "Above_IncludedLimit",
			Total: limit.FreeMax + 1,
			Usage: proto.AccessUsage{ValidCompute: _CU - 1, OverCompute: 1},
			Event: proto.Ptr(proto.EventType_FreeMax),
		}, {
			Name:  "Above_IncludedLimit_Exact",
			Total: limit.FreeMax + _CU,
			Usage: proto.AccessUsage{OverCompute: _CU},
			Event: nil,
		},
		// Overage Alert
		{
			Name:  "Within_OverageAlert",
			Total: limit.OverWarn - 1,
			Usage: proto.AccessUsage{OverCompute: _CU},
			Event: nil,
		}, {
			Name:  "Within_OverageAlert_Exact",
			Total: limit.OverWarn,
			Usage: proto.AccessUsage{OverCompute: _CU},
			Event: proto.Ptr(proto.EventType_OverWarn),
		}, {
			Name:  "Above_OverageAlert",
			Total: limit.OverWarn + 2,
			Usage: proto.AccessUsage{OverCompute: _CU},
			Event: proto.Ptr(proto.EventType_OverWarn),
		}, {
			Name:  "Above_OverageAlert_Exact",
			Total: limit.OverWarn + _CU,
			Usage: proto.AccessUsage{OverCompute: _CU},
			Event: nil,
		},
		// Overage Limit
		{
			Name:  "Within_OverageLimit",
			Total: limit.OverMax - 1,
			Usage: proto.AccessUsage{OverCompute: _CU},
			Event: nil,
		}, {
			Name:  "Above_OverageLimit_Exact",
			Total: limit.OverMax,
			Usage: proto.AccessUsage{OverCompute: _CU},
			Event: proto.Ptr(proto.EventType_OverMax),
		}, {
			Name:  "Above_OverageLimit",
			Total: limit.OverMax + 2,
			Usage: proto.AccessUsage{OverCompute: _CU - 2, LimitedCompute: 2},
			Event: proto.Ptr(proto.EventType_OverMax),
		}, {
			Name:  "Above_OverageLimit_Exact",
			Total: limit.OverMax + _CU,
			Usage: proto.AccessUsage{LimitedCompute: _CU},
			Event: nil,
		},
	} {
		t.Run(tc.Name, func(t *testing.T) {
			u, evt := limit.GetSpendResult(_CU, tc.Total)
			assert.Equal(t, tc.Usage, u)
			if tc.Event == nil {
				assert.Nil(t, evt)
				return
			}
			require.NotNil(t, evt)
			assert.Equal(t, tc.Event.String(), evt.String())
		})
	}

	// edge cases
	t.Run("EdgeCase", func(t *testing.T) {
		t.Run("NoFreeWarn", func(t *testing.T) {
			// it works for freeMax and 0
			limit.FreeWarn = limit.FreeMax
			u, evt := limit.GetSpendResult(1, limit.FreeMax)
			assert.Equal(t, proto.AccessUsage{ValidCompute: 1}, u)
			assert.Equal(t, proto.EventType_FreeMax.String(), evt.String())
			limit.FreeWarn = 0
			u, evt = limit.GetSpendResult(1, limit.FreeMax)
			assert.Equal(t, proto.AccessUsage{ValidCompute: 1}, u)
			assert.Equal(t, proto.EventType_FreeMax.String(), evt.String())
		})
		t.Run("NoOverWarn", func(t *testing.T) {
			// it works for overMax and 0
			limit.OverWarn = limit.OverMax
			u, evt := limit.GetSpendResult(1, limit.OverMax)
			assert.Equal(t, proto.AccessUsage{OverCompute: 1}, u)
			assert.Equal(t, proto.EventType_OverMax.String(), evt.String())
			limit.OverWarn = 0
			u, evt = limit.GetSpendResult(1, limit.OverMax)
			assert.Equal(t, proto.AccessUsage{OverCompute: 1}, u)
			assert.Equal(t, proto.EventType_OverMax.String(), evt.String())
		})
	})
}

func TestValidateLimit(t *testing.T) {
	assert.NoError(t, proto.Limit{RateLimit: 1, FreeMax: 2, OverMax: 2}.Validate())
	assert.NoError(t, proto.Limit{RateLimit: 1, FreeMax: 2, OverMax: 4}.Validate())
	assert.NoError(t, proto.Limit{RateLimit: 1, FreeWarn: 1, FreeMax: 2, OverWarn: 3, OverMax: 4}.Validate())

	assert.Error(t, proto.Limit{}.Validate())
	assert.Error(t, proto.Limit{RateLimit: 1}.Validate())
	assert.Error(t, proto.Limit{RateLimit: 1, FreeMax: 1}.Validate())
	assert.Error(t, proto.Limit{RateLimit: 1, FreeMax: 2, OverMax: 1}.Validate())
	assert.Error(t, proto.Limit{RateLimit: 1, FreeWarn: 3, FreeMax: 2, OverMax: 4}.Validate())
	assert.Error(t, proto.Limit{RateLimit: 1, FreeWarn: 1, FreeMax: 2, OverWarn: 5, OverMax: 4}.Validate())
}

func TestLimitJSON(t *testing.T) {
	expected := proto.Limit{
		RateLimit: 2000000000,
		FreeWarn:  20000000000,
		FreeMax:   20000000000,
		OverWarn:  2000000000000,
		OverMax:   math.MaxInt64,
	}
	expectedJson := `{
		"rateLimit": 2000000000,
		"freeWarn": 20000000000,
		"freeMax": 20000000000,
		"overWarn": 2000000000000,
		"overMax": 9223372036854775807,
		"freeCU": 20000000000,
		"softQuota": 2000000000000,
		"hardQuota": 9223372036854775807
	}`

	rawJson, err := json.Marshal(&expected)
	require.NoError(t, err)
	assert.JSONEq(t, expectedJson, string(rawJson))

	limit := proto.Limit{}
	err = json.Unmarshal(rawJson, &limit)
	require.NoError(t, err)
	assert.Equal(t, expected, limit)

	type LegacyLimit struct {
		RateLimit int64 `json:"rateLimit"`
		FreeCU    int64 `json:"freeCU"`
		SoftQuota int64 `json:"softQuota"`
		HardQuota int64 `json:"hardQuota"`
	}

	legacyExpected := LegacyLimit{
		RateLimit: 2000000000,
		FreeCU:    20000000000,
		SoftQuota: 2000000000000,
		HardQuota: math.MaxInt64,
	}
	legacyLimit := LegacyLimit{}
	err = json.Unmarshal(rawJson, &legacyLimit)
	require.NoError(t, err)
	assert.Equal(t, legacyExpected, legacyLimit)
}
