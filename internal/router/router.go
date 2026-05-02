package router

import (
	"github.com/gin-gonic/gin"
	"github.com/your-username/forum/internal/handler"
	"github.com/your-username/forum/internal/middleware"
	"net/http"
	"time"
)

func Setup(mode string) *gin.Engine {
	if mode == gin.ReleaseMode {
		gin.SetMode(gin.ReleaseMode)
	}
	r := gin.New()
	r.Use(middleware.TracingMiddleware(), gin.Logger(), gin.Recovery(), middleware.RateLimitMiddleware(time.Second, 100))

	v1 := r.Group("/api/v1")

	// User auth
	v1.POST("/signup", handler.SignUpHandler)
	v1.POST("/login", handler.LoginHandler)

	v1.Use(middleware.JWTAuthMiddleware())
	{
		v1.POST("/post", handler.CreatePostHandler)
		v1.GET("/post/:id", handler.GetPostByIDHandler)
		v1.GET("/posts", handler.GetPostListHandler)

		v1.POST("/vote", handler.VoteHandler)
		v1.POST("/comment", handler.CreateCommentHandler)
		v1.GET("/post/:post_id/comments", handler.GetCommentListHandler)

		// Video
		v1.POST("/video/upload", handler.UploadVideoHandler)
		v1.DELETE("/video/:id", handler.DeleteVideoHandler)
	}

	r.NoRoute(func(c *gin.Context) {
		c.JSON(http.StatusNotFound, gin.H{"msg": "404"})
	})

	return r
}
