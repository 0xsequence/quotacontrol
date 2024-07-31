package middleware

type RateLimiterConfig struct {
	Enabled                  bool   `toml:"enabled"`
	PublicRequestsPerMinute  int    `toml:"public_requests_per_minute"`
	UserRequestsPerMinute    int    `toml:"user_requests_per_minute"`
	ServiceRequestsPerMinute int    `toml:"service_requests_per_minute"`
	ErrorMessage             string `toml:"error_message"`
}
