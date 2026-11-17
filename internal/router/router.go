package router

import (
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/your-username/forum/config"
	"github.com/your-username/forum/internal/handler"
	"github.com/your-username/forum/internal/middleware"
	"github.com/your-username/forum/internal/pkg/metrics"
	"net/http"
)

func Setup(mode string, rateLimit *config.RateLimitConfig) *gin.Engine {
	if mode == gin.ReleaseMode {
		gin.SetMode(gin.ReleaseMode)
	}
	r := gin.New()
	r.Use(middleware.TracingMiddleware(), middleware.MetricsMiddleware(), middleware.CORSMiddleware(), gin.Logger(), gin.Recovery(), middleware.RateLimitMiddleware(rateLimit))
	r.GET("/metrics", gin.WrapH(metrics.Handler()))

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
		v1.GET("/comments/submissions/:client_id", handler.GetCommentSubmissionHandler)
		v1.GET("/post/:id/comments", handler.GetCommentListHandler)

		// Direct-message lab: follow graph + one-to-one conversations.
		v1.POST("/users/:id/follow", handler.FollowUserHandler)
		v1.DELETE("/users/:id/follow", handler.UnfollowUserHandler)
		v1.GET("/users/:id/relation", handler.GetUserRelationHandler)
		v1.POST("/conversations", handler.CreateConversationHandler)
		v1.GET("/conversations", handler.GetConversationListHandler)
		v1.GET("/conversations/:id/messages", handler.GetDirectMessagesHandler)
		v1.POST("/conversations/:id/messages", handler.SendDirectMessageHandler)
		v1.PUT("/conversations/:id/read", handler.MarkConversationReadHandler)
		v1.GET("/messages/unread-count", handler.GetDirectMessageUnreadCountHandler)
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
