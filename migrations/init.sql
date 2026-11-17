CREATE TABLE `users` (
    `id` bigint(20) NOT NULL AUTO_INCREMENT,
    `username` varchar(64) COLLATE utf8mb4_general_ci NOT NULL,
    `password` varchar(64) COLLATE utf8mb4_general_ci NOT NULL,
    `email` varchar(64) COLLATE utf8mb4_general_ci DEFAULT NULL,
    `create_time` timestamp NULL DEFAULT CURRENT_TIMESTAMP,
    `update_time` timestamp NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    PRIMARY KEY (`id`),
    UNIQUE KEY `idx_username` (`username`) USING BTREE,
    UNIQUE KEY `idx_email` (`email`) USING BTREE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_general_ci;

CREATE TABLE `posts` (
    `id` bigint(20) NOT NULL AUTO_INCREMENT,
    `author_id` bigint(20) NOT NULL,
    `title` varchar(128) COLLATE utf8mb4_general_ci NOT NULL,
    `content` text COLLATE utf8mb4_general_ci NOT NULL,
    `community_id` bigint(20) NOT NULL,
    `status` tinyint(4) NOT NULL DEFAULT '1',
    `post_type` tinyint(4) NOT NULL DEFAULT '1' COMMENT '1:text 2:link 3:video',
    `video_url` varchar(512) DEFAULT '' COMMENT 'Minio object URL',
    `thumbnail_url` varchar(512) DEFAULT '' COMMENT 'video thumbnail',
    `create_time` timestamp NULL DEFAULT CURRENT_TIMESTAMP,
    `update_time` timestamp NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    PRIMARY KEY (`id`),
    KEY `idx_author_id` (`author_id`),
    KEY `idx_community_id` (`community_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_general_ci;

CREATE TABLE `votes` (
    `id` bigint(20) NOT NULL AUTO_INCREMENT,
    `user_id` bigint(20) NOT NULL,
    `post_id` bigint(20) NOT NULL,
    `direction` tinyint(4) NOT NULL COMMENT '1:upvote, -1:downvote',
    `create_time` timestamp NULL DEFAULT CURRENT_TIMESTAMP,
    `update_time` timestamp NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    PRIMARY KEY (`id`),
    UNIQUE KEY `idx_user_post` (`user_id`, `post_id`),
    KEY `idx_post_id` (`post_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_general_ci;

CREATE TABLE `comments` (
    `id` bigint(20) NOT NULL AUTO_INCREMENT,
    `content` text COLLATE utf8mb4_general_ci NOT NULL,
    `author_id` bigint(20) NOT NULL,
    `post_id` bigint(20) NOT NULL,
    `parent_id` bigint(20) NOT NULL DEFAULT '0' COMMENT '0 for top-level, >0 for reply',
    `status` tinyint(4) NOT NULL DEFAULT '1',
    `create_time` timestamp NULL DEFAULT CURRENT_TIMESTAMP,
    `update_time` timestamp NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    PRIMARY KEY (`id`),
    KEY `idx_post_id` (`post_id`),
    KEY `idx_author_id` (`author_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_general_ci;

CREATE TABLE comment_submissions (
    author_id BIGINT NOT NULL,
    client_comment_id VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
    comment_id BIGINT NULL,
    PRIMARY KEY (author_id, client_comment_id),
    UNIQUE KEY idx_submission_comment (comment_id)
) ENGINE=InnoDB;

CREATE TABLE comment_submission_failures (
    author_id BIGINT NOT NULL,
    client_comment_id VARBINARY(64) NOT NULL,
    fingerprint BINARY(64) NOT NULL,
    PRIMARY KEY (author_id, client_comment_id, fingerprint)
) ENGINE=InnoDB;

CREATE TABLE `comment_votes` (
    `id` bigint(20) NOT NULL AUTO_INCREMENT,
    `user_id` bigint(20) NOT NULL,
    `comment_id` bigint(20) NOT NULL,
    `direction` tinyint(4) NOT NULL COMMENT '1:up,-1:down',
    `create_time` timestamp NULL DEFAULT CURRENT_TIMESTAMP,
    `update_time` timestamp NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    PRIMARY KEY (`id`),
    UNIQUE KEY `idx_user_comment` (`user_id`, `comment_id`),
    KEY `idx_comment_id` (`comment_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_general_ci;

CREATE TABLE `videos` (
    `id` bigint(20) NOT NULL AUTO_INCREMENT,
    `user_id` bigint(20) NOT NULL,
    `title` varchar(128) COLLATE utf8mb4_general_ci NOT NULL,
    `file_name` varchar(256) COLLATE utf8mb4_general_ci NOT NULL,
    `size` bigint(20) NOT NULL,
    `url` varchar(512) COLLATE utf8mb4_general_ci NOT NULL,
    `create_time` timestamp NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (`id`),
    KEY `idx_user_id` (`user_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_general_ci;

-- Direct-message lab -------------------------------------------------------
-- A directional follow edge. Two active opposite edges mean mutual follow.
CREATE TABLE `follows` (
    `id` bigint(20) NOT NULL AUTO_INCREMENT,
    `follower_id` bigint(20) NOT NULL,
    `followee_id` bigint(20) NOT NULL,
    `status` tinyint(4) NOT NULL DEFAULT '1',
    `create_time` timestamp NULL DEFAULT CURRENT_TIMESTAMP,
    `update_time` timestamp NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    PRIMARY KEY (`id`),
    UNIQUE KEY `idx_follower_followee` (`follower_id`, `followee_id`),
    KEY `idx_followee_status` (`followee_id`, `status`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_general_ci;

-- user_low_id/user_high_id are normalized in Go so one pair has one row.
CREATE TABLE `conversations` (
    `id` bigint(20) NOT NULL AUTO_INCREMENT,
    `user_low_id` bigint(20) NOT NULL,
    `user_high_id` bigint(20) NOT NULL,
    `last_message_id` bigint(20) NOT NULL DEFAULT '0',
    `last_message_time` timestamp NULL DEFAULT CURRENT_TIMESTAMP,
    `create_time` timestamp NULL DEFAULT CURRENT_TIMESTAMP,
    `update_time` timestamp NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    PRIMARY KEY (`id`),
    UNIQUE KEY `idx_conversation_pair` (`user_low_id`, `user_high_id`),
    KEY `idx_low_last_time` (`user_low_id`, `last_message_time`),
    KEY `idx_high_last_time` (`user_high_id`, `last_message_time`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_general_ci;

CREATE TABLE `direct_messages` (
    `id` bigint(20) NOT NULL AUTO_INCREMENT,
    `conversation_id` bigint(20) NOT NULL,
    `sender_id` bigint(20) NOT NULL,
    `receiver_id` bigint(20) NOT NULL,
    `client_message_id` varchar(64) COLLATE utf8mb4_general_ci NOT NULL,
    `content` text COLLATE utf8mb4_general_ci NOT NULL,
    `message_type` tinyint(4) NOT NULL DEFAULT '1' COMMENT '1:text',
    `status` tinyint(4) NOT NULL DEFAULT '1' COMMENT '1:sent,2:delivered,3:read,0:deleted',
    `create_time` timestamp NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (`id`),
    UNIQUE KEY `idx_sender_client_message` (`sender_id`, `client_message_id`),
    KEY `idx_conversation_message` (`conversation_id`, `id`),
    KEY `idx_receiver_unread` (`receiver_id`, `status`, `id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_general_ci;

CREATE TABLE `direct_message_outbox` (
    `id` bigint(20) NOT NULL AUTO_INCREMENT,
    `event_type` varchar(64) NOT NULL,
    `conversation_id` bigint(20) NOT NULL,
    `message_id` bigint(20) NOT NULL,
    `receiver_id` bigint(20) NOT NULL,
    `user_ids` json NOT NULL,
    `status` tinyint(4) NOT NULL DEFAULT '0' COMMENT '0:pending,1:processing,2:done',
    `attempts` int NOT NULL DEFAULT '0',
    `available_at` timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP,
    `locked_at` timestamp NULL DEFAULT NULL,
    `lock_token` varchar(64) NOT NULL DEFAULT '',
    `processed_at` timestamp NULL DEFAULT NULL,
    `last_error` varchar(512) NOT NULL DEFAULT '',
    `create_time` timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (`id`),
    KEY `idx_outbox_poll` (`status`, `available_at`, `id`),
    KEY `idx_outbox_lock` (`status`, `locked_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_general_ci;
