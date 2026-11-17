-- 先在现有数据库执行；回执与评论必须在同一事务中提交。
CREATE TABLE comment_submissions (
    author_id BIGINT NOT NULL,
    client_comment_id VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
    comment_id BIGINT NULL,
    PRIMARY KEY (author_id, client_comment_id),
    UNIQUE KEY idx_submission_comment (comment_id)
) ENGINE=InnoDB;
