package middleware

import (
	"github.com/gin-gonic/gin"
	"github.com/your-username/forum/internal/pkg/jwt"
	"net/http"
	"strings"
)

const ContextUserIDKey = "userID"

func JWTAuthMiddleware() func(c *gin.Context) {
	return func(c *gin.Context) {
		authHeader := c.Request.Header.Get("Authorization")
		if authHeader == "" {
			c.JSON(http.StatusUnauthorized, gin.H{"msg": "Authorization header is empty"})
			c.Abort()
			return
		}
		parts := strings.SplitN(authHeader, " ", 2)
		if !(len(parts) == 2 && parts[0] == "Bearer") {
			c.JSON(http.StatusUnauthorized, gin.H{"msg": "Authorization header format error"})
			c.Abort()
			return
		}
		mc, err := jwt.ParseToken(parts[1])
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"msg": "Invalid Token"})
			c.Abort()
			return
		}
		c.Set(ContextUserIDKey, mc.UserID)
		c.Next()
	}
}
