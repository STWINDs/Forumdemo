package router

import (
	"strings"

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
	r.Use(middleware.TracingMiddleware(), middleware.CORSMiddleware(), gin.Logger(), gin.Recovery(), middleware.RateLimitMiddleware(time.Second, 100))

	v1 := r.Group("/api/v1")

	v1.POST("/signup", handler.SignUpHandler)
	v1.POST("/login", handler.LoginHandler)

	v1.Use(middleware.JWTAuthMiddleware())
	{
		v1.POST("/post", handler.CreatePostHandler)
		v1.GET("/post/:id", handler.GetPostByIDHandler)
		v1.PUT("/post/:id", handler.UpdatePostHandler)
		v1.DELETE("/post/:id", handler.DeletePostHandler)
		v1.GET("/posts", handler.GetPostListHandler)

		v1.POST("/vote", handler.VoteHandler)
		v1.POST("/comment-vote", handler.CommentVoteHandler)
		v1.GET("/post/:id/votes", handler.GetPostVotesHandler)
		v1.POST("/comment", handler.CreateCommentHandler)
		v1.GET("/post/:id/comments", handler.GetCommentListHandler)
	}

	r.Static("/static", "./web")

	r.NoRoute(func(c *gin.Context) {
		if !strings.HasPrefix(c.Request.URL.Path, "/api/") {
			c.File("./web/index.html")
			return
		}
		c.JSON(http.StatusNotFound, gin.H{"msg": "404"})
	})

	return r
}
