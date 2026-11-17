CREATE TABLE comment_submission_failures (
    author_id BIGINT NOT NULL,
    client_comment_id VARBINARY(64) NOT NULL,
    fingerprint BINARY(64) NOT NULL,
    PRIMARY KEY (author_id, client_comment_id, fingerprint)
) ENGINE=InnoDB;
