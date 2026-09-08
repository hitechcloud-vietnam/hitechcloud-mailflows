package middleware

import (
	"time"

	"github.com/gin-gonic/gin"
)

func CORS(allowedOrigins []string) gin.HandlerFunc {
	return func(c *gin.Context) {
		origin := c.Request.Header.Get("Origin")

		allowed := false
		for _, o := range allowedOrigins {
			if o == "*" || o == origin {
				allowed = true
				break
			}
		}

		if allowed {
			c.Header("Access-Control-Allow-Origin", origin)
		}
		c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
		c.Header("Access-Control-Allow-Headers", "Origin, Content-Type, Accept, Authorization, X-Requested-With")
		c.Header("Access-Control-Allow-Credentials", "true")
		c.Header("Access-Control-Max-Age", "86400")

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}

		c.Next()
	}
}

func SecurityHeaders() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("X-Content-Type-Options", "nosniff")
		c.Header("X-Frame-Options", "SAMEORIGIN")
		c.Header("Referrer-Policy", "strict-origin-when-cross-origin")
		c.Header("X-XSS-Protection", "1; mode=block")
		c.Header("Permissions-Policy", "camera=(), microphone=(), geolocation=()")
		c.Next()
	}
}

func RateLimit(requests int, window time.Duration) gin.HandlerFunc {
	// Simple in-memory rate limiter - production should use Redis
	type visitor struct {
		count    int
		lastSeen time.Time
	}

	visitors := make(map[string]*visitor)

	go func() {
		for {
			time.Sleep(window)
			for ip, v := range visitors {
				if time.Since(v.lastSeen) > window {
					delete(visitors, ip)
				}
			}
		}
	}()

	return func(c *gin.Context) {
		ip := c.ClientIP()
		v, exists := visitors[ip]
		if !exists {
			visitors[ip] = &visitor{count: 1, lastSeen: time.Now()}
			c.Next()
			return
		}

		if time.Since(v.lastSeen) < window {
			if v.count >= requests {
				c.JSON(429, gin.H{"error": "Rate limit exceeded"})
				c.Abort()
				return
			}
			v.count++
		} else {
			v.count = 1
		}
		v.lastSeen = time.Now()

		c.Next()
	}
}
