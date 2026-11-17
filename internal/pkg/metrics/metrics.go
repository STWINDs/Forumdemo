// Package metrics provides a small dependency-free Prometheus text exporter.
// Labels are deliberately bounded: HTTP routes must be Gin route templates,
// never raw paths containing IDs.
package metrics

import (
	"context"
	"database/sql"
	"fmt"
	"net/http"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

var durationBuckets = []float64{0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10}

type histogram struct {
	Count   uint64
	Sum     float64
	Buckets []uint64
}

func newHistogram() *histogram {
	return &histogram{Buckets: make([]uint64, len(durationBuckets))}
}

func (h *histogram) observe(value float64) {
	h.Count++
	h.Sum += value
	for i, bound := range durationBuckets {
		if value <= bound {
			h.Buckets[i]++
		}
	}
}

type httpKey struct {
	Method string
	Route  string
	Status int
}

type OutboxStats struct {
	Pending             int64
	Processing          int64
	OldestPendingSecond int64
}

var registry = struct {
	sync.RWMutex
	http           map[httpKey]*histogram
	cache          map[string]uint64
	rateLimit      map[string]uint64
	database       map[string]uint64
	outbox         map[string]uint64
	outboxBatch    *histogram
	dbProvider     func() sql.DBStats
	outboxProvider func(context.Context) (OutboxStats, error)
}{
	http:        make(map[httpKey]*histogram),
	cache:       make(map[string]uint64),
	rateLimit:   make(map[string]uint64),
	database:    make(map[string]uint64),
	outbox:      make(map[string]uint64),
	outboxBatch: newHistogram(),
}

var httpInFlight atomic.Int64

func HTTPInFlightAdd(delta int64) { httpInFlight.Add(delta) }

func ObserveHTTP(method, route string, status int, duration time.Duration) {
	if route == "" {
		route = "unmatched"
	}
	key := httpKey{Method: method, Route: route, Status: status}
	registry.Lock()
	h := registry.http[key]
	if h == nil {
		h = newHistogram()
		registry.http[key] = h
	}
	h.observe(duration.Seconds())
	registry.Unlock()
}

func IncCache(result string)       { increment(registry.cache, result) }
func IncRateLimit(decision string) { increment(registry.rateLimit, decision) }
func IncDatabase(result string)    { increment(registry.database, result) }
func IncOutbox(result string)      { increment(registry.outbox, result) }

func increment(counter map[string]uint64, key string) {
	registry.Lock()
	counter[key]++
	registry.Unlock()
}

func ObserveOutboxBatch(processed int, duration time.Duration) {
	registry.Lock()
	registry.outbox["batch_events"] += uint64(processed)
	registry.outboxBatch.observe(duration.Seconds())
	registry.Unlock()
}

func SetDBStatsProvider(provider func() sql.DBStats) {
	registry.Lock()
	registry.dbProvider = provider
	registry.Unlock()
}

func SetOutboxStatsProvider(provider func(context.Context) (OutboxStats, error)) {
	registry.Lock()
	registry.outboxProvider = provider
	registry.Unlock()
}

func Handler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain; version=0.0.4; charset=utf-8")
		_, _ = w.Write([]byte(Render(r.Context())))
	})
}

func Render(ctx context.Context) string {
	registry.RLock()
	httpSnapshot := make(map[httpKey]histogram, len(registry.http))
	for key, value := range registry.http {
		httpSnapshot[key] = histogram{Count: value.Count, Sum: value.Sum, Buckets: append([]uint64(nil), value.Buckets...)}
	}
	cacheSnapshot := cloneCounterMap(registry.cache)
	rateSnapshot := cloneCounterMap(registry.rateLimit)
	databaseSnapshot := cloneCounterMap(registry.database)
	outboxSnapshot := cloneCounterMap(registry.outbox)
	batchSnapshot := histogram{Count: registry.outboxBatch.Count, Sum: registry.outboxBatch.Sum, Buckets: append([]uint64(nil), registry.outboxBatch.Buckets...)}
	dbProvider := registry.dbProvider
	outboxProvider := registry.outboxProvider
	registry.RUnlock()

	var output strings.Builder
	writeMetric(&output, "forum_http_in_flight_requests", nil, float64(httpInFlight.Load()))

	httpKeys := make([]httpKey, 0, len(httpSnapshot))
	for key := range httpSnapshot {
		httpKeys = append(httpKeys, key)
	}
	sort.Slice(httpKeys, func(i, j int) bool {
		if httpKeys[i].Route != httpKeys[j].Route {
			return httpKeys[i].Route < httpKeys[j].Route
		}
		if httpKeys[i].Method != httpKeys[j].Method {
			return httpKeys[i].Method < httpKeys[j].Method
		}
		return httpKeys[i].Status < httpKeys[j].Status
	})
	for _, key := range httpKeys {
		labels := map[string]string{"method": key.Method, "route": key.Route, "status": strconv.Itoa(key.Status)}
		writeMetric(&output, "forum_http_requests_total", labels, float64(httpSnapshot[key].Count))
		writeHistogram(&output, "forum_http_request_duration_seconds", labels, httpSnapshot[key])
	}

	writeCounterMap(&output, "forum_cache_requests_total", "result", cacheSnapshot)
	writeCounterMap(&output, "forum_rate_limit_requests_total", "decision", rateSnapshot)
	writeCounterMap(&output, "forum_database_operations_total", "result", databaseSnapshot)
	writeCounterMap(&output, "forum_outbox_events_total", "result", outboxSnapshot)
	writeHistogram(&output, "forum_outbox_batch_duration_seconds", nil, batchSnapshot)

	var mem runtime.MemStats
	runtime.ReadMemStats(&mem)
	writeMetric(&output, "forum_go_goroutines", nil, float64(runtime.NumGoroutine()))
	writeMetric(&output, "forum_go_heap_alloc_bytes", nil, float64(mem.HeapAlloc))
	writeMetric(&output, "forum_go_heap_inuse_bytes", nil, float64(mem.HeapInuse))
	writeMetric(&output, "forum_go_gc_cycles_total", nil, float64(mem.NumGC))

	if dbProvider != nil {
		stats := dbProvider()
		writeMetric(&output, "forum_mysql_pool_max_open_connections", nil, float64(stats.MaxOpenConnections))
		writeMetric(&output, "forum_mysql_pool_open_connections", nil, float64(stats.OpenConnections))
		writeMetric(&output, "forum_mysql_pool_in_use_connections", nil, float64(stats.InUse))
		writeMetric(&output, "forum_mysql_pool_idle_connections", nil, float64(stats.Idle))
		writeMetric(&output, "forum_mysql_pool_wait_total", nil, float64(stats.WaitCount))
		writeMetric(&output, "forum_mysql_pool_wait_duration_seconds_total", nil, stats.WaitDuration.Seconds())
	}

	if outboxProvider != nil {
		stats, err := outboxProvider(ctx)
		if err != nil {
			writeMetric(&output, "forum_outbox_stats_up", nil, 0)
		} else {
			writeMetric(&output, "forum_outbox_stats_up", nil, 1)
			writeMetric(&output, "forum_outbox_pending", nil, float64(stats.Pending))
			writeMetric(&output, "forum_outbox_processing", nil, float64(stats.Processing))
			writeMetric(&output, "forum_outbox_oldest_pending_seconds", nil, float64(stats.OldestPendingSecond))
		}
	}
	return output.String()
}

func cloneCounterMap(source map[string]uint64) map[string]uint64 {
	result := make(map[string]uint64, len(source))
	for key, value := range source {
		result[key] = value
	}
	return result
}

func writeCounterMap(output *strings.Builder, name, label string, values map[string]uint64) {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		writeMetric(output, name, map[string]string{label: key}, float64(values[key]))
	}
}

func writeHistogram(output *strings.Builder, name string, labels map[string]string, value histogram) {
	for i, bound := range durationBuckets {
		bucketLabels := cloneLabels(labels)
		bucketLabels["le"] = strconv.FormatFloat(bound, 'g', -1, 64)
		writeMetric(output, name+"_bucket", bucketLabels, float64(value.Buckets[i]))
	}
	infLabels := cloneLabels(labels)
	infLabels["le"] = "+Inf"
	writeMetric(output, name+"_bucket", infLabels, float64(value.Count))
	writeMetric(output, name+"_sum", labels, value.Sum)
	writeMetric(output, name+"_count", labels, float64(value.Count))
}

func writeMetric(output *strings.Builder, name string, labels map[string]string, value float64) {
	output.WriteString(name)
	if len(labels) > 0 {
		keys := make([]string, 0, len(labels))
		for key := range labels {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		output.WriteByte('{')
		for i, key := range keys {
			if i > 0 {
				output.WriteByte(',')
			}
			fmt.Fprintf(output, `%s="%s"`, key, escapeLabel(labels[key]))
		}
		output.WriteByte('}')
	}
	output.WriteByte(' ')
	output.WriteString(strconv.FormatFloat(value, 'g', -1, 64))
	output.WriteByte('\n')
}

func cloneLabels(source map[string]string) map[string]string {
	result := make(map[string]string, len(source)+1)
	for key, value := range source {
		result[key] = value
	}
	return result
}

func escapeLabel(value string) string {
	value = strings.ReplaceAll(value, `\`, `\\`)
	value = strings.ReplaceAll(value, "\n", `\n`)
	return strings.ReplaceAll(value, `"`, `\"`)
}
