package handler

import (
	"fmt"
	"path"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/your-username/forum/config"
	"github.com/your-username/forum/internal/middleware"
	"github.com/your-username/forum/internal/model"
	"github.com/your-username/forum/internal/pkg/minio"
	"github.com/your-username/forum/internal/service"
	"net/http"
)

func CreatePostHandler(c *gin.Context) {
	userID, ok := c.Get(middleware.ContextUserIDKey)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"msg": "user not logged in"})
		return
	}

	// Accept multipart/form-data for video posts, JSON for text/link
	contentType := c.Request.Header.Get("Content-Type")
	var p model.Post

	if len(contentType) >= 9 && contentType[:9] == "multipart" {
		p.Title = c.PostForm("title")
		p.Content = c.PostForm("content")
		postTypeStr := c.PostForm("post_type")
		communityStr := c.PostForm("community_id")
		if postTypeStr == "" {
			postTypeStr = "1"
		}
		if communityStr == "" {
			communityStr = "1"
		}
		p.PostType = int8(parseInt(postTypeStr))
		p.CommunityID = parseInt64(communityStr)

		// Handle video upload
		if p.PostType == 3 {
			file, header, err := c.Request.FormFile("video")
			if err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"msg": "video file required for video posts"})
				return
			}
			defer file.Close()

			ext := path.Ext(header.Filename)
			objName := fmt.Sprintf("%d_%s%s", userID, uuid.New().String(), ext)
			_, err = minio.UploadFile(c.Request.Context(), config.Conf.Minio.BucketName, objName, file, header.Size, "video/mp4")
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"msg": "video upload failed: " + err.Error()})
				return
			}
			p.VideoURL = objName
		}
	} else {
		if err := c.ShouldBindJSON(&p); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"msg": "invalid params"})
			return
		}
		if p.PostType == 0 {
			p.PostType = 1
		}
	}

	if p.Title == "" || p.Content == "" {
		c.JSON(http.StatusBadRequest, gin.H{"msg": "title and content required"})
		return
	}

	p.AuthorID = userID.(int64)
	if err := service.CreatePost(&p); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"msg": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"msg": "success"})
}

func GetPostByIDHandler(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"msg": "invalid id"})
		return
	}
	post, err := service.GetPostByID(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"msg": err.Error()})
		return
	}
	c.JSON(http.StatusOK, post)
}

func GetPostListHandler(c *gin.Context) {
	pageStr := c.DefaultQuery("page", "1")
	sizeStr := c.DefaultQuery("size", "10")
	page, _ := strconv.ParseInt(pageStr, 10, 64)
	size, _ := strconv.ParseInt(sizeStr, 10, 64)

	posts, err := service.GetPostListWithVotes(page, size)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"msg": err.Error()})
		return
	}
	c.JSON(http.StatusOK, posts)
}

func UpdatePostHandler(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"msg": "invalid id"})
		return
	}
	userID, ok := c.Get(middleware.ContextUserIDKey)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"msg": "user not logged in"})
		return
	}

	// Accept JSON body for title/content/post_type
	var body struct {
		Title    string `json:"title"`
		Content  string `json:"content"`
		PostType int8   `json:"post_type"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"msg": "invalid params"})
		return
	}
	if body.PostType == 0 {
		body.PostType = 1
	}

	if err := service.UpdatePost(id, userID.(int64), body.Title, body.Content, body.PostType); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"msg": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"msg": "updated"})
}

func DeletePostHandler(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"msg": "invalid id"})
		return
	}
	userID, ok := c.Get(middleware.ContextUserIDKey)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"msg": "user not logged in"})
		return
	}

	if err := service.DeletePost(id, userID.(int64)); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"msg": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"msg": "deleted"})
}

func parseInt(s string) int64 {
	n, _ := strconv.ParseInt(s, 10, 64)
	return n
}

func parseInt64(s string) int64 {
	n, _ := strconv.ParseInt(s, 10, 64)
	return n
}
