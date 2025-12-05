package middleware

import (
	"batch-embedding-api/config"
	"batch-embedding-api/models"
	"net/http"
	"strings"
	"sync"

	"github.com/gin-gonic/gin"
	"golang.org/x/time/rate"
)

// AuthMiddleware validates API keys and RapidAPI headers
func AuthMiddleware(cfg *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Check for RapidAPI proxy secret first
		if cfg.RapidAPIProxySecret != "" {
			rapidAPISecret := c.GetHeader("X-RapidAPI-Proxy-Secret")
			if rapidAPISecret == cfg.RapidAPIProxySecret {
				// Request is from RapidAPI
				c.Set("auth_type", "rapidapi")
				c.Set("rapidapi_user", c.GetHeader("X-RapidAPI-User"))
				c.Next()
				return
			}
		}

		// Check for Bearer token
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, models.Error{
				Code:    "unauthorized",
				Message: "Missing Authorization header",
			})
			return
		}

		// Extract token
		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) != 2 || strings.ToLower(parts[0]) != "bearer" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, models.Error{
				Code:    "unauthorized",
				Message: "Invalid Authorization header format. Use: Bearer <token>",
			})
			return
		}

		token := parts[1]

		// Validate token against allowed API keys
		valid := false
		for _, key := range cfg.APIKeys {
			if token == strings.TrimSpace(key) {
				valid = true
				break
			}
		}

		if !valid {
			c.AbortWithStatusJSON(http.StatusUnauthorized, models.Error{
				Code:    "unauthorized",
				Message: "Invalid API key",
			})
			return
		}

		c.Set("auth_type", "api_key")
		c.Next()
	}
}

// RateLimiter implements per-client rate limiting
type RateLimiter struct {
	clients map[string]*rate.Limiter
	mutex   sync.RWMutex
	rate    rate.Limit
	burst   int
}

// NewRateLimiter creates a new rate limiter
func NewRateLimiter(rps int, burst int) *RateLimiter {
	return &RateLimiter{
		clients: make(map[string]*rate.Limiter),
		rate:    rate.Limit(rps),
		burst:   burst,
	}
}

// GetLimiter returns the rate limiter for a client
func (r *RateLimiter) GetLimiter(clientID string) *rate.Limiter {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	if limiter, exists := r.clients[clientID]; exists {
		return limiter
	}

	limiter := rate.NewLimiter(r.rate, r.burst)
	r.clients[clientID] = limiter
	return limiter
}

// RateLimitMiddleware applies rate limiting
func RateLimitMiddleware(limiter *RateLimiter) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Use API key or IP as client identifier
		clientID := c.GetHeader("X-RapidAPI-User")
		if clientID == "" {
			clientID = c.GetHeader("Authorization")
		}
		if clientID == "" {
			clientID = c.ClientIP()
		}

		if !limiter.GetLimiter(clientID).Allow() {
			c.AbortWithStatusJSON(http.StatusTooManyRequests, models.Error{
				Code:    "too_many_requests",
				Message: "Rate limit exceeded. Please slow down.",
			})
			return
		}

		c.Next()
	}
}

// ErrorHandler provides consistent error response handling
func ErrorHandler() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Next()

		// Handle any errors that occurred
		if len(c.Errors) > 0 {
			err := c.Errors.Last()
			c.JSON(http.StatusInternalServerError, models.Error{
				Code:    "internal_error",
				Message: err.Error(),
			})
		}
	}
}

// CORSMiddleware adds CORS headers
func CORSMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("Access-Control-Allow-Origin", "*")
		c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		c.Header("Access-Control-Allow-Headers", "Origin, Content-Type, Authorization, X-RapidAPI-Key, X-RapidAPI-Host")

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}

		c.Next()
	}
}
