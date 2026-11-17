package mysql

import "context"

// GetDirectMessageOutboxStats returns exact backlog gauges. The status index
// bounds the scan to pending and processing rows instead of completed history.
func GetDirectMessageOutboxStats(ctx context.Context) (pending, processing, oldestPendingSeconds int64, err error) {
	if err = isReady(); err != nil {
		return
	}
	err = db.QueryRowxContext(ctx, `SELECT
		COALESCE(SUM(status = 0), 0),
		COALESCE(SUM(status = 1), 0),
		COALESCE(TIMESTAMPDIFF(SECOND, MIN(CASE WHEN status = 0 THEN create_time END), NOW()), 0)
		FROM direct_message_outbox WHERE status IN (0, 1)`).Scan(&pending, &processing, &oldestPendingSeconds)
	return
}
