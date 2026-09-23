package router_test

import (
	"github.com/lp/campus-market/internal/config"
)

// testConfig builds a permissive config for route-level smoke tests.
func testConfig() *config.Config {
	return &config.Config{
		Port:            "8080",
		DSN:             "",
		JWTSecret:       "test-secret-key-for-smoke-tests-only",
		JWTExpireHours:  24,
		RateLimitPerMin: 100000,
		LoginRateLimit:  100000,
		CORSOrigins:     []string{"*"},
		SeedingEnabled:  false,
	}
}
