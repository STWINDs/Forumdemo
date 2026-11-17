CREATE TABLE IF NOT EXISTS `direct_message_outbox` (
    `id` bigint(20) NOT NULL AUTO_INCREMENT,
    `event_type` varchar(64) NOT NULL,
    `conversation_id` bigint(20) NOT NULL,
    `message_id` bigint(20) NOT NULL,
    `user_ids` json NOT NULL,
    `status` tinyint(4) NOT NULL DEFAULT '0',
    `attempts` int NOT NULL DEFAULT '0',
    `available_at` timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP,
    `locked_at` timestamp NULL DEFAULT NULL,
    `processed_at` timestamp NULL DEFAULT NULL,
    `last_error` varchar(512) NOT NULL DEFAULT '',
    `create_time` timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (`id`),
    KEY `idx_outbox_poll` (`status`, `available_at`, `id`),
    KEY `idx_outbox_lock` (`status`, `locked_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_general_ci;
