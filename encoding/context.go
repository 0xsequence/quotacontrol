package encoding

import "context"

type contextKey struct {
	name string
}

func (k *contextKey) String() string {
	return "encoding context value " + k.name
}

var (
	ctxKeyPrefix  = &contextKey{"Prefix"}
	ctxKeyVersion = &contextKey{"Version"}
)

// WithPrefix sets the prefix to the context.
func WithPrefix(ctx context.Context, prefix string) context.Context {
	return context.WithValue(ctx, ctxKeyPrefix, prefix)
}

// getPrefix returns the prefix from the context. If not set, it returns DefaultPrefix.
func getPrefix(ctx context.Context) string {
	v, _ := ctx.Value(ctxKeyPrefix).(string)
	if v == "" {
		return DefaultPrefix
	}
	return v
}

// WithVersion sets the version to the context.
func WithVersion(ctx context.Context, version byte) context.Context {
	return context.WithValue(ctx, ctxKeyVersion, version)
}

// GetVersion returns the version from the context. If not set, it returns AccessKeyVersion.
func GetVersion(ctx context.Context) (byte, bool) {
	v, ok := ctx.Value(ctxKeyVersion).(byte)
	return v, ok
}
