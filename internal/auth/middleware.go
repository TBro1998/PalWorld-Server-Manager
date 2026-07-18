package auth

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

// Middleware returns a Gin handler that enforces JWT authentication.
//
// The token is read from two sources (first match wins):
//  1. Authorization: Bearer <token> request header  (standard API calls)
//  2. ?token=<token> query parameter                 (EventSource / SSE, which
//     cannot set custom headers in browsers)
func Middleware(secret string) gin.HandlerFunc {
	return func(c *gin.Context) {
		token := extractToken(c)
		if token == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "authentication required"})
			return
		}
		if err := ValidateToken(token, secret); err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "invalid or expired token"})
			return
		}
		c.Next()
	}
}

// extractToken pulls the raw token string from the request, checking the
// Authorization header first and the ?token query param as a fallback.
func extractToken(c *gin.Context) string {
	if h := c.GetHeader("Authorization"); h != "" {
		parts := strings.SplitN(h, " ", 2)
		if len(parts) == 2 && strings.EqualFold(parts[0], "Bearer") {
			return parts[1]
		}
	}
	return c.Query("token")
}
