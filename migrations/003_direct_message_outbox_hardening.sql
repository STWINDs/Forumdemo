-- Upgrade an outbox table previously created by 002_direct_message_outbox.sql.
-- receiver_id is backfilled from the second element of the existing user_ids
-- payload, whose order is [sender_id, receiver_id].
ALTER TABLE `direct_message_outbox`
    ADD COLUMN `receiver_id` bigint(20) NULL AFTER `message_id`,
    ADD COLUMN `lock_token` varchar(64) NOT NULL DEFAULT '' AFTER `locked_at`;

UPDATE `direct_message_outbox`
SET `receiver_id` = CAST(JSON_UNQUOTE(JSON_EXTRACT(`user_ids`, '$[1]')) AS UNSIGNED)
WHERE `receiver_id` IS NULL;

ALTER TABLE `direct_message_outbox`
    MODIFY COLUMN `receiver_id` bigint(20) NOT NULL;
