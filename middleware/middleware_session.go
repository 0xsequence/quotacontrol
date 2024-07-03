package middleware

import (
	"net/http"
	"strings"

	"github.com/0xsequence/quotacontrol/proto"
)

// Session middleware that detects the session type and sets the account or service on the context.
func Session() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return session{Next: next}
	}
}

// AccessControl middleware that checks if the session type is allowed to access the endpoint.
// It also sets the compute units on the context if the endpoint requires it.
func AccessControl(acl ACL, cost Cost, defaultCost int64) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return accessControl{
			ACL:         acl,
			Cost:        cost,
			DefaultCost: defaultCost,
			Next:        next,
		}
	}
}

type session struct {
	Next http.Handler
}

func (m session) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	_, claims, ok := getJWT(ctx)
	if !ok {
		sessionType := proto.SessionType_Public
		if _, ok := GetAccessQuota(ctx); ok {
			sessionType = proto.SessionType_AccessKey
		}
		ctx = withSessionType(ctx, sessionType)
		m.Next.ServeHTTP(w, r.WithContext(ctx))
		return
	}

	// Origin check
	if originClaim, ok := claims["ogn"].(string); ok {
		originClaim = strings.TrimSuffix(originClaim, "/")
		originHeader := strings.TrimSuffix(r.Header.Get("Origin"), "/")
		if originHeader != "" && originHeader != originClaim {
			proto.RespondWithError(w, proto.ErrUnauthorized.WithCausef("invalid origin claim"))
			return
		}
	}

	// Set account or service on the context
	accountClaim, _ := claims["account"].(string)
	serviceClaim, _ := claims["service"].(string)
	adminClaim, _ := claims["admin"].(bool)

	switch quota, ok := GetAccessQuota(ctx); {
	case serviceClaim != "":
		ctx = withSessionType(ctx, proto.SessionType_Service)
		ctx = withService(ctx, serviceClaim)
	case adminClaim:
		if accountClaim == "" {
			proto.RespondWithError(w, proto.ErrUnauthorized)
			return
		}
		ctx = withAccount(ctx, accountClaim)
		ctx = withSessionType(ctx, proto.SessionType_Admin)
	case accountClaim != "":
		ctx = withAccount(ctx, accountClaim)
		sessionType := proto.SessionType_Account
		if ok {
			sessionType = proto.SessionType_AccessKey
			if quota.IsJWT() {
				sessionType = proto.SessionType_Project
			}
		}
		ctx = withSessionType(ctx, sessionType)
	default:
		sessionType := proto.SessionType_Public
		if ok {
			sessionType = proto.SessionType_AccessKey
		}
		ctx = withSessionType(ctx, sessionType)
	}

	m.Next.ServeHTTP(w, r.WithContext(ctx))
}

type accessControl struct {
	ACL         ACL
	Cost        Cost
	DefaultCost int64
	Next        http.Handler
}

func (m accessControl) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	req := newRequest(r.URL.Path)
	if req == nil {
		proto.RespondWithError(w, proto.ErrUnauthorized.WithCausef("invalid rpc method called"))
		return
	}

	min, ok := m.ACL.GetConfig(req)
	if !ok {
		proto.RespondWithError(w, proto.ErrUnauthorized.WithCausef("rpc method not found"))
		return
	}

	if session := GetSessionType(r.Context()); session < min {
		proto.RespondWithError(w, proto.ErrUnauthorized)
		return
	}

	ctx := r.Context()

	credits := m.DefaultCost
	if v, ok := m.Cost.GetConfig(req); ok {
		credits = v
	}
	ctx = WithComputeUnits(ctx, credits)

	m.Next.ServeHTTP(w, r.WithContext(ctx))
}
