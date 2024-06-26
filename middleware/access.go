package middleware

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/0xsequence/quotacontrol/proto"

	"github.com/go-chi/jwtauth/v5"
	"github.com/lestrrat-go/jwx/v2/jwt"
)

// SetCredentials gets the credentials from the JWT or the Access key.
// It uses `jwtauth.Verifier` to get the JWT, extra  the project claim and sets it in the context as well.
// When both are present, it checks project mismatch between the access key and the JWT token.
func SetCredentials(ja *jwtauth.JWTAuth, accessKeyFuncs ...func(*http.Request) string) func(next http.Handler) http.Handler {
	baseFuncs := []func(*http.Request) string{
		func(r *http.Request) string { return r.Header.Get(HeaderAccessKey) },
	}
	return func(next http.Handler) http.Handler {
		return credentials{Auth: ja, KeyFuncs: append(baseFuncs, accessKeyFuncs...), Next: next}
	}
}

// VerifyQuota checks if the project is in the context and fetches the quota for the project.
// If it's not, but an access key is present, it fetches the quota for the access key.
func VerifyQuota(client Client) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return verify{Client: client, Next: next}
	}
}

// EnsureUsage is a middleware that checks if the quota has enough usage left.
func EnsureUsage(client Client) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return ensure{Client: client, Next: next}
	}
}

// SpendUsage is a middleware that spends the usage from the quota.
func SpendUsage(client Client) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return spend{Client: client, Next: next}
	}
}

func EnsurePermission(client Client, minPermission proto.UserPermission) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return permission{Client: client, Next: next, Perm: minPermission}
	}
}

type credentials struct {
	Auth     *jwtauth.JWTAuth
	KeyFuncs []func(*http.Request) string
	Next     http.Handler
}

func (m credentials) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	var accessKey string
	for _, f := range m.KeyFuncs {
		if accessKey = f(r); accessKey != "" {
			break
		}
	}

	ctx := r.Context()

	var projectID uint64
	if accessKey != "" {
		ctx = WithAccessKey(ctx, accessKey)
	}

	token, err := jwtauth.VerifyRequest(m.Auth, r, jwtauth.TokenFromHeader, jwtauth.TokenFromCookie)
	if err != nil {
		if errors.Is(err, jwtauth.ErrExpired) {
			proto.RespondWithError(w, proto.ErrSessionExpired)
			return
		}
		if errors.Is(err, jwtauth.ErrNoTokenFound) {
			m.Next.ServeHTTP(w, r.WithContext(ctx))
			return
		}
		proto.RespondWithError(w, proto.ErrUnauthorizedUser)
		return
	}

	if err := jwt.Validate(token, m.Auth.ValidateOptions()...); err != nil {
		proto.RespondWithError(w, err)
		return
	}

	claims, err := token.AsMap(r.Context())
	if err != nil {
		proto.RespondWithError(w, err)
		return
	}

	ctx = withJWT(ctx, token, claims)

	if v, ok := claims["project"].(float64); ok {
		projectID = uint64(v)
		if accessKey != "" {
			if id, _ := proto.GetProjectID(accessKey); id != projectID {
				proto.RespondWithError(w, proto.ErrAccessKeyMismatch)
				return
			}
		}
	}

	if projectID != 0 {
		ctx = withProjectID(ctx, projectID)
	}

	if account, ok := claims["account"].(string); ok {
		ctx = withAccount(ctx, account)
	}

	m.Next.ServeHTTP(w, r.WithContext(ctx))
}

type verify struct {
	Client Client
	Next   http.Handler
}

func (m verify) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if !m.Client.IsEnabled() {
		m.Next.ServeHTTP(w, r)
		return
	}

	var quota *proto.AccessQuota

	ctx := r.Context()
	now := GetTime(ctx)

	// check if we alreayd have a project ID from the JWT
	if projectID, ok := getProjectID(ctx); ok {
		q, err := m.Client.FetchProjectQuota(ctx, projectID, now)
		if err != nil {
			proto.RespondWithError(w, err)
			return
		}

		perm, _, err := m.Client.FetchPermission(ctx, projectID, GetAccount(ctx), true)
		if err != nil {
			proto.RespondWithError(w, err)
			return
		}
		if !perm.CanAccess(proto.UserPermission_READ) {
			proto.RespondWithError(w, proto.ErrUnauthorizedUser)
			return
		}

		quota = q
	}

	// check if we have an access key
	if accessKey := GetAccessKey(ctx); accessKey != "" {
		q, err := m.Client.FetchKeyQuota(ctx, accessKey, r.Header.Get(HeaderOrigin), now)
		if err != nil {
			proto.RespondWithError(w, err)
			return
		}
		// don't override the quota from the project
		if quota == nil {
			quota = q
		}
	}

	if quota != nil {
		ctx = withAccessQuota(ctx, quota)
		w.Header().Set(HeaderQuotaLimit, strconv.FormatInt(quota.Limit.FreeMax, 10))
	}

	m.Next.ServeHTTP(w, r.WithContext(ctx))
}

type ensure struct {
	Client Client
	Next   http.Handler
}

func (m ensure) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	quota := GetAccessQuota(ctx)
	if quota == nil {
		m.Next.ServeHTTP(w, r)
		return
	}

	cu, ok := getComputeUnits(ctx)
	if !ok {
		cu = m.Client.GetDefaultUsage()
	}
	if cu == 0 {
		m.Next.ServeHTTP(w, r)
		return
	}

	usage, err := m.Client.FetchUsage(ctx, quota, GetTime(ctx))
	if err != nil {
		proto.RespondWithError(w, err)
		return
	}
	w.Header().Set(HeaderQuotaRemaining, strconv.FormatInt(max(quota.Limit.FreeMax-usage, 0), 10))
	if overage := max(usage-quota.Limit.FreeMax, 0); overage > 0 {
		w.Header().Set(HeaderQuotaOverage, strconv.FormatInt(overage, 10))
	}
	if usage+cu > quota.Limit.OverMax {
		proto.RespondWithError(w, proto.ErrLimitExceeded)
		return
	}

	m.Next.ServeHTTP(w, r)
}

type spend struct {
	Client Client
	Next   http.Handler
}

func (m spend) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if !m.Client.IsEnabled() {
		m.Next.ServeHTTP(w, r)
		return
	}

	ctx := r.Context()

	quota := GetAccessQuota(ctx)
	if quota == nil {
		m.Next.ServeHTTP(w, r)
		return
	}

	cu, ok := getComputeUnits(ctx)
	if !ok {
		cu = m.Client.GetDefaultUsage()
	}
	if cu == 0 {
		m.Next.ServeHTTP(w, r)
		return
	}

	ok, total, err := m.Client.SpendQuota(ctx, quota, cu, GetTime(ctx))
	if err != nil && !errors.Is(err, proto.ErrLimitExceeded) {
		proto.RespondWithError(w, err)
		return
	}

	w.Header().Set(HeaderQuotaRemaining, strconv.FormatInt(max(quota.Limit.FreeMax-total, 0), 10))
	if overage := total - quota.Limit.FreeMax; overage > 0 {
		w.Header().Set(HeaderQuotaOverage, strconv.FormatInt(overage, 10))
	}

	if errors.Is(err, proto.ErrLimitExceeded) {
		proto.RespondWithError(w, err)
		return
	}

	if ok {
		ctx = withSpending(ctx)
	}

	m.Next.ServeHTTP(w, r.WithContext(ctx))
}

type permission struct {
	Client Client
	Next   http.Handler
	Perm   proto.UserPermission
}

func (m permission) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if !m.Client.IsEnabled() {
		m.Next.ServeHTTP(w, r)
		return
	}

	ctx := r.Context()

	// check if we alreayd have a project ID from the JWT
	q := GetAccessQuota(ctx)
	if q == nil || !q.IsJWT() {
		proto.RespondWithError(w, proto.ErrUnauthorizedUser)
		return
	}

	perm, _, err := m.Client.FetchPermission(ctx, q.GetProjectID(), GetAccount(ctx), true)
	if err != nil || !perm.CanAccess(m.Perm) {
		proto.RespondWithError(w, proto.ErrUnauthorizedUser)
		return
	}

	m.Next.ServeHTTP(w, r)
}
