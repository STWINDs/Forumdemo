package metrics

import (
	"context"
	"database/sql"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestRenderUsesBoundedRouteLabelsAndProviders(t *testing.T) {
	ObserveHTTP("GET", "/api/v1/post/:id", 200, 12*time.Millisecond)
	IncCache("l1_hit")
	IncRateLimit("allowed")
	IncDatabase("retry")
	IncOutbox("created")
	ObserveOutboxBatch(10, 25*time.Millisecond)
	SetDBStatsProvider(func() sql.DBStats {
		return sql.DBStats{MaxOpenConnections: 100, OpenConnections: 5, InUse: 2, Idle: 3}
	})
	SetOutboxStatsProvider(func(context.Context) (OutboxStats, error) {
		return OutboxStats{Pending: 7, Processing: 1, OldestPendingSecond: 9}, nil
	})

	output := Render(context.Background())
	for _, expected := range []string{
		`forum_http_requests_total{method="GET",route="/api/v1/post/:id",status="200"}`,
		`forum_cache_requests_total{result="l1_hit"}`,
		`forum_rate_limit_requests_total{decision="allowed"}`,
		`forum_database_operations_total{result="retry"}`,
		`forum_mysql_pool_in_use_connections 2`,
		`forum_outbox_pending 7`,
		`forum_outbox_stats_up 1`,
	} {
		require.True(t, strings.Contains(output, expected), "missing metric %s", expected)
	}
}
