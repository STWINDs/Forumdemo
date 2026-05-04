package service

import (
	"context"
	"strconv"
	"time"

	"github.com/your-username/forum/config"
	"github.com/your-username/forum/internal/dao/mysql"
	"github.com/your-username/forum/internal/dao/redis"
	"github.com/your-username/forum/internal/model"
	"github.com/your-username/forum/internal/pkg/minio"
)

func CreatePost(post *model.Post) (err error) {
	if err = mysql.CreatePost(post); err != nil {
		return err
	}
	return redis.CreatePost(post.ID)
}

func GetPostByID(ctx context.Context, id int64) (post *model.Post, err error) {
	post, err = redis.GetPostDetailWithCache(ctx, id)
	if err != nil {
		return nil, err
	}
	enrichVideoURL(post)
	return post, nil
}

func UpdatePost(id, authorID int64, title, content string, postType int8) error {
	if err := mysql.UpdatePost(id, authorID, title, content, postType); err != nil {
		return err
	}
	invalidatePostCache(id)
	return nil
}

func DeletePost(id, authorID int64) error {
	if err := mysql.DeletePost(id, authorID); err != nil {
		return err
	}
	invalidatePostCache(id)
	return nil
}

func invalidatePostCache(postID int64) {
	cacheKey := "forum:post:" + strconv.FormatInt(postID, 10)
	// Invalidate L1 local cache
	redis.DelL1(cacheKey)
	// Invalidate L2 Redis cache
	redis.DeleteCache(cacheKey)
}

func enrichVideoURL(post *model.Post) {
	if post.PostType == 3 && post.VideoURL != "" {
		url, err := minio.GetPresignedURL(context.Background(), config.Conf.Minio.BucketName, post.VideoURL, 24*time.Hour)
		if err == nil {
			post.VideoURL = url
		}
	}
}

func GetPostList(page, size int64) (posts []*model.Post, err error) {
	return mysql.GetPostList(page, size)
}

// PostSummary 帖子列表摘要（含赞数、评论数、热评）
type PostSummary struct {
	Post         *model.Post `json:"post"`
	Upvotes      int64       `json:"upvotes"`
	Downvotes    int64       `json:"downvotes"`
	CommentCount int64       `json:"comment_count"`
	TopComment   *TopComment `json:"top_comment,omitempty"`
}

type TopComment struct {
	ID       int64  `json:"id"`
	Content  string `json:"content"`
	AuthorID int64  `json:"author_id"`
	Upvotes  int64  `json:"upvotes"`
}

// GetPostListWithVotes 获取帖子列表并附加投票数和热评
func GetPostListWithVotes(page, size int64) ([]PostSummary, error) {
	posts, err := mysql.GetPostList(page, size)
	if err != nil {
		return nil, err
	}

	ids := make([]int64, len(posts))
	for i, p := range posts {
		ids[i] = p.ID
	}

	// Pipeline 批量获取赞数
	voteCounts, _ := redis.PipelineGetPostVoteCounts(ids)
	commentCounts, _ := mysql.GetCommentCounts(ids)
	topComments, _ := mysql.GetTopComments(ids)

	// Get top comment IDs for vote counts
	tcIDs := make([]string, 0)
	tcByPost := map[int64]string{}
	for pid, tc := range topComments {
		sid := strconv.FormatInt(tc.ID, 10)
		tcIDs = append(tcIDs, sid)
		tcByPost[pid] = sid
	}
	tcVotes, _ := redis.GetCommentVoteCounts(tcIDs)

	result := make([]PostSummary, len(posts))
	for i, p := range posts {
		enrichVideoURL(p)
		up, down := int64(0), int64(0)
		if vc, ok := voteCounts[p.ID]; ok && len(vc) >= 2 {
			up, down = vc[0], vc[1]
		}
		cc := int64(0)
		if c, ok := commentCounts[p.ID]; ok {
			cc = c
		}
		ps := PostSummary{
			Post:         p,
			Upvotes:      up,
			Downvotes:    down,
			CommentCount: cc,
		}
		if tc, ok := topComments[p.ID]; ok {
			tcUp := int64(0)
			if tcVotes != nil {
				if vc, ok := tcVotes[tcByPost[p.ID]]; ok && len(vc) >= 2 {
					tcUp = vc[0]
				}
			}
			ps.TopComment = &TopComment{
				ID:       tc.ID,
				Content:  truncateStr(tc.Content, 60),
				AuthorID: tc.AuthorID,
				Upvotes:  tcUp,
			}
		}
		result[i] = ps
	}
	return result, nil
}

func truncateStr(s string, max int) string {
	runes := []rune(s)
	if len(runes) <= max {
		return s
	}
	return string(runes[:max]) + "..."
}
