package middleware

import (
	"net/http"
)

// SecurityHeaders adds standard security headers to all responses
func SecurityHeaders() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// HSTS - Force HTTPS for 1 year, include subdomains
			w.Header().Set("Strict-Transport-Security", "max-age=31536000; includeSubDomains; preload")

			// Prevent MIME type sniffing
			w.Header().Set("X-Content-Type-Options", "nosniff")

			// Prevent clickjacking
			w.Header().Set("X-Frame-Options", "DENY")

			// XSS Protection (legacy, but still useful)
			w.Header().Set("X-XSS-Protection", "1; mode=block")

			// Referrer Policy - only send origin for cross-origin requests
			w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")

			// Permissions Policy - disable dangerous APIs
			w.Header().Set("Permissions-Policy", "geolocation=(), microphone=(), camera=()")

			// Content Security Policy
			// Adjust as needed for your frontend
			w.Header().Set("Content-Security-Policy",
				"default-src 'self'; "+
					"script-src 'self' 'unsafe-inline' https://apis.google.com; "+
					"style-src 'self' 'unsafe-inline'; "+
					"img-src 'self' data: https:; "+
					"font-src 'self' https://fonts.gstatic.com; "+
					"connect-src 'self' https://*.googleapis.com https://*.firebaseio.com; "+
					"frame-ancestors 'none'")

			next.ServeHTTP(w, r)
		})
	}
}

// SecureHeadersStrict is an even stricter version for API-only endpoints
func SecureHeadersStrict() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Strict-Transport-Security", "max-age=31536000; includeSubDomains; preload")
			w.Header().Set("X-Content-Type-Options", "nosniff")
			w.Header().Set("X-Frame-Options", "DENY")
			w.Header().Set("X-XSS-Protection", "1; mode=block")
			w.Header().Set("Referrer-Policy", "no-referrer")
			w.Header().Set("Content-Security-Policy", "default-src 'none'; frame-ancestors 'none'")
			w.Header().Set("Cache-Control", "no-store, no-cache, must-revalidate, private")
			w.Header().Set("Pragma", "no-cache")

			next.ServeHTTP(w, r)
		})
	}
}
