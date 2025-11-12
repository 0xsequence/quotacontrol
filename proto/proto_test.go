package proto_test

import (
	"encoding/json"
	"testing"

	"github.com/0xsequence/quotacontrol/proto"
	"github.com/goware/validation"
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
			AllowedOrigins: []validation.Origin{"http://localhost:8080", "http://localhost:8081", "http://localhost:8082/"},
		}
		assert.True(t, tk.ValidateOrigin("http://localhost:8080"))
		assert.True(t, tk.ValidateOrigin("http://localhost:8081"))
		assert.True(t, tk.ValidateOrigin("http://localhost:8082"))
		assert.False(t, tk.ValidateOrigin("http://localhost:8083"))
	})

	t.Run("match any", func(t *testing.T) {
		tk := &proto.AccessKey{
			AllowedOrigins: []validation.Origin{"*"},
		}
		assert.True(t, tk.ValidateOrigin("http://sequence.xyz"))
		assert.True(t, tk.ValidateOrigin("https://localhost:8080"))
	})

	t.Run("match http scheme", func(t *testing.T) {
		tk := &proto.AccessKey{
			AllowedOrigins: []validation.Origin{"https://localhost:8080", "https://*.sequence.xyz"},
		}
		assert.True(t, tk.ValidateOrigin("https://local.sequence.xyz"))
		assert.True(t, tk.ValidateOrigin("https://localhost:8080"))
		assert.False(t, tk.ValidateOrigin("http://localhost:8080"))
	})

	t.Run("wildcards", func(t *testing.T) {
		tk := &proto.AccessKey{
			AllowedOrigins: []validation.Origin{"*.sequence.xyz"},
		}
		assert.False(t, tk.ValidateOrigin("http://sequence.xyz"))
		assert.True(t, tk.ValidateOrigin("http://docs.sequence.xyz"))
		assert.True(t, tk.ValidateOrigin("http://test.sequence.xyz"))
		assert.True(t, tk.ValidateOrigin("http://dev.test.sequence.xyz"))
	})

	t.Run("empty origin", func(t *testing.T) {
		tk := &proto.AccessKey{
			AllowedOrigins: []validation.Origin{"http://localhost:8080", "*.sequence.xyz"},
		}
		assert.True(t, tk.ValidateOrigin(""))
	})

	t.Run("enforce origin", func(t *testing.T) {
		tk := &proto.AccessKey{
			RequireOrigin:  true,
			AllowedOrigins: []validation.Origin{},
		}
		assert.False(t, tk.ValidateOrigin(""))
		assert.True(t, tk.ValidateOrigin("http://localhost:8080"))
	})
}

func TestGetSpendResult(t *testing.T) {
	const (
		_CU = 5
	)
	limit := proto.ServiceLimit{
		FreeWarn: 10,
		FreeMax:  20,
		OverWarn: 30,
		OverMax:  40,
	}

	type TestCase struct {
		Name  string
		Total int64

		Usage int64
		Event *proto.EventType
	}

	for _, tc := range []TestCase{
		// Include Alert
		{
			Name:  "Within_IncludedAlert",
			Total: limit.FreeWarn - 1,
			Usage: _CU,
			Event: nil,
		},
		{
			Name:  "Within_IncludedAlert_Exact",
			Total: limit.FreeWarn,
			Usage: _CU,
			Event: proto.Ptr(proto.EventType_FreeWarn),
		},
		{
			Name:  "Above_IncludedAlert",
			Total: limit.FreeWarn + 1,
			Usage: _CU,
			Event: proto.Ptr(proto.EventType_FreeWarn),
		},
		{
			Name:  "Above_IncludedAlert_Exact",
			Total: limit.FreeWarn + _CU,
			Usage: _CU,
			Event: nil,
		},
		// Include Limit
		{
			Name:  "Within_IncludedLimit",
			Total: limit.FreeWarn - 1,
			Usage: _CU,
			Event: nil,
		},
		{
			Name:  "Within_IncludedLimit_Exact",
			Total: limit.FreeMax,
			Usage: _CU,
			Event: proto.Ptr(proto.EventType_FreeMax),
		},
		{
			Name:  "Above_IncludedLimit",
			Total: limit.FreeMax + 1,
			Usage: _CU,
			Event: proto.Ptr(proto.EventType_FreeMax),
		},
		{
			Name:  "Above_IncludedLimit_Exact",
			Total: limit.FreeMax + _CU,
			Usage: _CU,
			Event: nil,
		},
		// Overage Alert
		{
			Name:  "Within_OverageAlert",
			Total: limit.OverWarn - 1,
			Usage: _CU,
			Event: nil,
		},
		{
			Name:  "Within_OverageAlert_Exact",
			Total: limit.OverWarn,
			Usage: _CU,
			Event: proto.Ptr(proto.EventType_OverWarn),
		},
		{
			Name:  "Above_OverageAlert",
			Total: limit.OverWarn + 2,
			Usage: _CU,
			Event: proto.Ptr(proto.EventType_OverWarn),
		},
		{
			Name:  "Above_OverageAlert_Exact",
			Total: limit.OverWarn + _CU,
			Usage: _CU,
			Event: nil,
		},
		// Overage Limit
		{
			Name:  "Within_OverageLimit",
			Total: limit.OverMax - 1,
			Usage: _CU,
			Event: nil,
		},
		{
			Name:  "Above_OverageLimit_Exact",
			Total: limit.OverMax,
			Usage: _CU,
			Event: proto.Ptr(proto.EventType_OverMax),
		},
		{
			Name:  "Above_OverageLimit",
			Total: limit.OverMax + 2,
			Usage: _CU - 2,
			Event: proto.Ptr(proto.EventType_OverMax),
		},
		{
			Name:  "Above_OverageLimit_More",
			Total: limit.OverMax + _CU,
			Usage: 0,
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
			assert.Equal(t, int64(1), u)
			assert.Equal(t, proto.EventType_FreeMax.String(), evt.String())
			limit.FreeWarn = 0
			u, evt = limit.GetSpendResult(1, limit.FreeMax)
			assert.Equal(t, int64(1), u)
			assert.Equal(t, proto.EventType_FreeMax.String(), evt.String())
		})
		t.Run("NoOverWarn", func(t *testing.T) {
			// it works for overMax and 0
			limit.OverWarn = limit.OverMax
			u, evt := limit.GetSpendResult(1, limit.OverMax)
			assert.Equal(t, int64(1), u)
			assert.Equal(t, proto.EventType_OverMax.String(), evt.String())
			limit.OverWarn = 0
			u, evt = limit.GetSpendResult(1, limit.OverMax)
			assert.Equal(t, int64(1), u)
			assert.Equal(t, proto.EventType_OverMax.String(), evt.String())
		})
	})
}

func TestValidateLimit(t *testing.T) {
	assert.NoError(t, proto.ServiceLimit{RateLimit: 1, FreeMax: 2, OverMax: 2}.Validate())
	assert.NoError(t, proto.ServiceLimit{RateLimit: 1, FreeMax: 2, OverMax: 4}.Validate())
	assert.NoError(t, proto.ServiceLimit{RateLimit: 1, FreeWarn: 1, FreeMax: 2, OverWarn: 3, OverMax: 4}.Validate())

	assert.Error(t, proto.ServiceLimit{}.Validate())
	assert.Error(t, proto.ServiceLimit{RateLimit: 1}.Validate())
	assert.Error(t, proto.ServiceLimit{RateLimit: 1, FreeMax: 1}.Validate())
	assert.Error(t, proto.ServiceLimit{RateLimit: 1, FreeMax: 2, OverMax: 1}.Validate())
	assert.Error(t, proto.ServiceLimit{RateLimit: 1, FreeWarn: 3, FreeMax: 2, OverMax: 4}.Validate())
	assert.Error(t, proto.ServiceLimit{RateLimit: 1, FreeWarn: 1, FreeMax: 2, OverWarn: 5, OverMax: 4}.Validate())
}

func TestServiceName(t *testing.T) {
	for i := range proto.Service_name {
		svc := proto.Service(i)
		name := svc.GetName()
		assert.NotEqual(t, "", name, "Service %d should have a name", svc)
		t.Logf("Service %d: %s", i, name)
	}
}

func TestUnknownService(t *testing.T) {
	input := `
{
    "serviceLimit": {
        "API": {
            "rateLimit": 18000,
            "freeWarn": 300000000,
            "freeMax": 300000000,
            "overWarn": 300000000,
            "overMax": 9007199254740991
        },
        "Builder": {
            "rateLimit": 18000,
            "freeWarn": 300000000,
            "freeMax": 300000000,
            "overWarn": 300000000,
            "overMax": 9007199254740991
        },
        "Indexer": {
            "rateLimit": 18000,
            "freeWarn": 300000000,
            "freeMax": 300000000,
            "overWarn": 300000000,
            "overMax": 9007199254740991
        },
        "Marketplace": {
            "rateLimit": 18000,
            "freeWarn": 300000000,
            "freeMax": 300000000,
            "overWarn": 300000000,
            "overMax": 9007199254740991
        },
        "Metadata": {
            "rateLimit": 18000,
            "freeWarn": 300000000,
            "freeMax": 300000000,
            "overWarn": 300000000,
            "overMax": 9007199254740991
        },
        "NodeGateway": {
            "rateLimit": 18000,
            "freeWarn": 300000000,
            "freeMax": 300000000,
            "overWarn": 300000000,
            "overMax": 9007199254740991
        },
        "Relayer": {
            "rateLimit": 18000,
            "freeWarn": 300000000,
            "freeMax": 300000000,
            "overWarn": 300000000,
            "overMax": 9007199254740991
        },
        "Trails": {
            "rateLimit": 100000,
            "freeWarn": 100000000000,
            "freeMax": 100000000000,
            "overWarn": 100000000000,
            "overMax": 100000000000
        },
        "WaaS": {
            "rateLimit": 18000,
            "freeWarn": 300000000,
            "freeMax": 300000000,
            "overWarn": 300000000,
            "overMax": 9007199254740991
        },
		"Banana": {
            "rateLimit": 18000,
            "freeWarn": 300000000,
            "freeMax": 300000000,
            "overWarn": 300000000,
            "overMax": 9007199254740991
        }
    },
    "rateLimit": 244000,
    "freeWarn": 102400000000,
    "freeMax": 102400000000,
    "overWarn": 102400000000,
    "overMax": 72057694037927940
}
	`
	var l proto.Limit
	if err := json.Unmarshal([]byte(input), &l); err != nil {
		t.Fatal(err)
	}
	// Verify unknown service is included
	assert.Equal(t, int64(18000), l.ServiceLimit["Banana"].RateLimit)
}
