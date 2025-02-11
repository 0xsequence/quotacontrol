package proto_test

import (
	"testing"

	"github.com/0xsequence/quotacontrol/proto"
	"github.com/goware/validation"
	"github.com/stretchr/testify/assert"
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
	limit := proto.Limit{
		FreeWarn: 10,
		FreeMax:  20,
		OverWarn: 30,
		OverMax:  40,
	}

	type TestCase struct {
		Name  string
		Total int64

		Ok    bool
		Spent int64
		Event *proto.EventType
	}

	for _, tc := range []TestCase{
		// Include Alert
		{
			Name: "Within_IncludedAlert", Total: limit.FreeWarn - 1,
			Ok: true, Spent: _CU, Event: nil,
		},
		{
			Name: "Within_IncludedAlert_Exact", Total: limit.FreeWarn,
			Ok: true, Spent: _CU, Event: proto.Ptr(proto.EventType_FreeWarn),
		},
		{
			Name: "Above_IncludedAlert", Total: limit.FreeWarn + 1,
			Ok: true, Spent: _CU, Event: proto.Ptr(proto.EventType_FreeWarn),
		},
		{
			Name: "Above_IncludedAlert_Exact", Total: limit.FreeWarn + _CU,
			Ok: true, Spent: _CU, Event: nil,
		},
		// Include Limit
		{
			Name: "Within_IncludedLimit", Total: limit.FreeWarn - 1,
			Ok: true, Spent: _CU, Event: nil,
		},
		{
			Name: "Within_IncludedLimit_Exact", Total: limit.FreeMax,
			Ok: true, Spent: _CU, Event: proto.Ptr(proto.EventType_FreeMax),
		},
		{
			Name: "Above_IncludedLimit", Total: limit.FreeMax + 1,
			Ok: true, Spent: _CU, Event: proto.Ptr(proto.EventType_FreeMax),
		},
		{
			Name: "Above_IncludedLimit_Exact", Total: limit.FreeMax + _CU,
			Ok: true, Spent: _CU, Event: nil,
		},
		// Overage Alert
		{
			Name: "Within_OverageAlert", Total: limit.OverWarn - 1,
			Ok: true, Spent: _CU, Event: nil,
		},
		{
			Name: "Within_OverageAlert_Exact", Total: limit.OverWarn,
			Ok: true, Spent: _CU, Event: proto.Ptr(proto.EventType_OverWarn),
		},
		{
			Name: "Above_OverageAlert", Total: limit.OverWarn + 2,
			Ok: true, Spent: _CU, Event: proto.Ptr(proto.EventType_OverWarn),
		},
		{
			Name: "Above_OverageAlert_Exact", Total: limit.OverWarn + _CU,
			Ok: true, Spent: _CU, Event: nil,
		},
		// Overage Limit
		{
			Name: "Within_OverageLimit", Total: limit.OverMax - 1,
			Ok: true, Spent: _CU, Event: nil,
		},
		{
			Name: "Above_OverageLimit_Exact", Total: limit.OverMax,
			Ok: true, Spent: _CU, Event: proto.Ptr(proto.EventType_OverMax),
		},
		{
			Name: "Above_OverageLimit", Total: limit.OverMax + 2,
			Ok: false, Spent: 3, Event: proto.Ptr(proto.EventType_OverMax),
		},
		{
			Name: "Above_OverageLimit_More", Total: limit.OverMax + _CU,
			Ok: false, Spent: 0, Event: nil,
		},
	} {
		t.Run(tc.Name, func(t *testing.T) {
			ok, u, evt := limit.GetSpendResult(_CU, tc.Total)
			assert.Equal(t, tc.Ok, ok)
			assert.Equal(t, tc.Spent, u)
			assert.Equal(t, tc.Event, evt)
		})
	}

	// edge cases
	t.Run("EdgeCase", func(t *testing.T) {
		t.Run("NoFreeWarn", func(t *testing.T) {
			// it works for freeMax and 0
			limit.FreeWarn = limit.FreeMax
			ok, u, evt := limit.GetSpendResult(1, limit.FreeMax)
			assert.True(t, ok)
			assert.Equal(t, int64(1), u)
			assert.Equal(t, proto.EventType_FreeMax, *evt)
			limit.FreeWarn = 0
			ok, u, evt = limit.GetSpendResult(1, limit.FreeMax)
			assert.Equal(t, int64(1), u)
			assert.Equal(t, proto.EventType_FreeMax, *evt)
		})
		t.Run("NoOverWarn", func(t *testing.T) {
			// it works for overMax and 0
			limit.OverWarn = limit.OverMax

			ok, u, evt := limit.GetSpendResult(1, limit.OverMax)
			assert.True(t, ok)
			assert.Equal(t, int64(1), u)
			assert.Equal(t, proto.EventType_OverMax, *evt)

			ok, u, evt = limit.GetSpendResult(2, limit.OverMax+1)
			assert.False(t, ok)
			assert.Equal(t, int64(1), u)
			assert.Equal(t, proto.EventType_OverMax, *evt)

			limit.OverWarn = 0

			ok, u, evt = limit.GetSpendResult(1, limit.OverMax)
			assert.True(t, ok)
			assert.Equal(t, int64(1), u)
			assert.Equal(t, proto.EventType_OverMax, *evt)

			ok, u, evt = limit.GetSpendResult(2, limit.OverMax+1)
			assert.False(t, ok)
			assert.Equal(t, int64(1), u)
			assert.Equal(t, proto.EventType_OverMax, *evt)
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
