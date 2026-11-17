// Command benchload drives requests that hey cannot model because each direct
// message needs a unique client_message_id. It also provides a deterministic
// mixed workload matching the project's initial benchmark profile.
package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

type settings struct {
	baseURL        string
	token          string
	mode           string
	postID         int64
	targetUserID   int64
	conversationID int64
	qps            float64
	duration       time.Duration
	concurrency    int
	timeout        time.Duration
}

type operation string

const (
	opHotPost        operation = "hot_post"
	opPostList       operation = "post_list"
	opComments       operation = "comments"
	opConversations  operation = "conversations"
	opFollow         operation = "follow"
	opMessageHistory operation = "message_history"
	opUnread         operation = "unread"
	opSendMessage    operation = "send_message"
)

type result struct {
	op       operation
	status   int
	duration time.Duration
	err      error
}

type operationStats struct {
	total     int
	success   int
	clientErr int
	serverErr int
	limited   int
	errors    int
	latencies []time.Duration
}

var messageSequence atomic.Uint64

func main() {
	cfg := parseFlags()
	if err := validate(cfg); err != nil {
		fmt.Fprintln(os.Stderr, "configuration error:", err)
		os.Exit(2)
	}

	started := time.Now()
	stats := run(cfg)
	elapsed := time.Since(started)
	printSummary(cfg, stats, elapsed)
}

func parseFlags() settings {
	var cfg settings
	flag.StringVar(&cfg.baseURL, "base-url", "http://127.0.0.1:8080", "Forumdemo base URL")
	flag.StringVar(&cfg.token, "token", "", "JWT bearer token (or BENCH_TOKEN env)")
	flag.StringVar(&cfg.mode, "mode", "mixed", "mixed or dm")
	flag.Int64Var(&cfg.postID, "post-id", 0, "existing post ID")
	flag.Int64Var(&cfg.targetUserID, "target-user-id", 0, "existing follow target user ID")
	flag.Int64Var(&cfg.conversationID, "conversation-id", 0, "existing conversation ID")
	flag.Float64Var(&cfg.qps, "qps", 50, "total scheduled requests per second")
	flag.DurationVar(&cfg.duration, "duration", time.Minute, "load duration")
	flag.IntVar(&cfg.concurrency, "concurrency", 20, "worker count")
	flag.DurationVar(&cfg.timeout, "timeout", 10*time.Second, "per-request timeout")
	flag.Parse()
	if cfg.token == "" {
		cfg.token = os.Getenv("BENCH_TOKEN")
	}
	cfg.baseURL = strings.TrimRight(cfg.baseURL, "/")
	return cfg
}

func validate(cfg settings) error {
	if cfg.token == "" {
		return fmt.Errorf("token or BENCH_TOKEN is required")
	}
	if cfg.mode != "mixed" && cfg.mode != "dm" {
		return fmt.Errorf("mode must be mixed or dm")
	}
	if cfg.postID <= 0 || cfg.targetUserID <= 0 || cfg.conversationID <= 0 {
		return fmt.Errorf("post-id, target-user-id and conversation-id must be positive")
	}
	if cfg.qps <= 0 || cfg.duration <= 0 || cfg.concurrency <= 0 || cfg.timeout <= 0 {
		return fmt.Errorf("qps, duration, concurrency and timeout must be positive")
	}
	return nil
}

func run(cfg settings) map[operation]*operationStats {
	jobs := make(chan operation, cfg.concurrency*2)
	results := make(chan result, cfg.concurrency*2)
	client := &http.Client{
		Timeout: cfg.timeout,
		Transport: &http.Transport{
			MaxIdleConns:        cfg.concurrency * 2,
			MaxIdleConnsPerHost: cfg.concurrency,
			MaxConnsPerHost:     cfg.concurrency,
			IdleConnTimeout:     90 * time.Second,
		},
	}

	var workers sync.WaitGroup
	for i := 0; i < cfg.concurrency; i++ {
		workers.Add(1)
		go func() {
			defer workers.Done()
			for op := range jobs {
				results <- execute(client, cfg, op)
			}
		}()
	}

	stats := make(map[operation]*operationStats)
	aggregated := make(chan struct{})
	go func() {
		defer close(aggregated)
		for item := range results {
			entry := stats[item.op]
			if entry == nil {
				entry = new(operationStats)
				stats[item.op] = entry
			}
			entry.total++
			entry.latencies = append(entry.latencies, item.duration)
			if item.err != nil {
				entry.errors++
				continue
			}
			switch {
			case item.status >= 200 && item.status < 300:
				entry.success++
			case item.status == http.StatusTooManyRequests:
				entry.limited++
			case item.status >= 500:
				entry.serverErr++
			default:
				entry.clientErr++
			}
		}
	}()

	interval := time.Duration(float64(time.Second) / cfg.qps)
	ticker := time.NewTicker(interval)
	deadline := time.NewTimer(cfg.duration)
	sequence := 0
schedule:
	for {
		select {
		case <-deadline.C:
			break schedule
		case <-ticker.C:
			jobs <- chooseOperation(cfg.mode, sequence)
			sequence++
		}
	}
	ticker.Stop()
	close(jobs)
	workers.Wait()
	close(results)
	<-aggregated
	return stats
}

func chooseOperation(mode string, sequence int) operation {
	if mode == "dm" {
		return opSendMessage
	}
	percent := sequence % 100
	switch {
	case percent < 20:
		return opHotPost
	case percent < 40:
		return opPostList
	case percent < 60:
		return opComments
	case percent < 75:
		return opConversations
	case percent < 85:
		return opFollow
	case percent < 90:
		return opMessageHistory
	case percent < 95:
		return opUnread
	default:
		return opSendMessage
	}
}

func execute(client *http.Client, cfg settings, op operation) result {
	method, path := http.MethodGet, ""
	var body io.Reader
	switch op {
	case opHotPost:
		path = "/api/v1/post/" + strconv.FormatInt(cfg.postID, 10)
	case opPostList:
		path = "/api/v1/posts?page=1&size=10"
	case opComments:
		path = "/api/v1/post/" + strconv.FormatInt(cfg.postID, 10) + "/comments"
	case opConversations:
		path = "/api/v1/conversations?limit=30"
	case opFollow:
		method = http.MethodPost
		path = "/api/v1/users/" + strconv.FormatInt(cfg.targetUserID, 10) + "/follow"
	case opMessageHistory:
		path = "/api/v1/conversations/" + strconv.FormatInt(cfg.conversationID, 10) + "/messages?limit=30"
	case opUnread:
		path = "/api/v1/messages/unread-count"
	case opSendMessage:
		method = http.MethodPost
		path = "/api/v1/conversations/" + strconv.FormatInt(cfg.conversationID, 10) + "/messages"
		payload, _ := json.Marshal(map[string]string{
			"client_message_id": fmt.Sprintf("bench-%d-%d", time.Now().UnixNano(), messageSequence.Add(1)),
			"content":           "Forumdemo benchmark message",
		})
		body = bytes.NewReader(payload)
	}

	request, err := http.NewRequest(method, cfg.baseURL+path, body)
	if err != nil {
		return result{op: op, err: err}
	}
	request.Header.Set("Authorization", "Bearer "+cfg.token)
	if body != nil {
		request.Header.Set("Content-Type", "application/json")
	}
	started := time.Now()
	response, err := client.Do(request)
	duration := time.Since(started)
	if err != nil {
		return result{op: op, duration: duration, err: err}
	}
	_, _ = io.Copy(io.Discard, response.Body)
	_ = response.Body.Close()
	return result{op: op, status: response.StatusCode, duration: duration}
}

func printSummary(cfg settings, stats map[operation]*operationStats, elapsed time.Duration) {
	var total operationStats
	for _, item := range stats {
		total.total += item.total
		total.success += item.success
		total.clientErr += item.clientErr
		total.serverErr += item.serverErr
		total.limited += item.limited
		total.errors += item.errors
		total.latencies = append(total.latencies, item.latencies...)
	}
	fmt.Printf("mode=%s duration=%s configured_qps=%.2f concurrency=%d\n", cfg.mode, elapsed.Round(time.Millisecond), cfg.qps, cfg.concurrency)
	fmt.Printf("total=%d success=%d http_4xx=%d http_429=%d http_5xx=%d transport_errors=%d\n",
		total.total, total.success, total.clientErr, total.limited, total.serverErr, total.errors)
	fmt.Printf("success_qps=%.2f error_rate=%.4f p50=%s p95=%s p99=%s\n",
		float64(total.success)/elapsed.Seconds(), failureRate(total), percentile(total.latencies, 0.50), percentile(total.latencies, 0.95), percentile(total.latencies, 0.99))

	operations := make([]string, 0, len(stats))
	for op := range stats {
		operations = append(operations, string(op))
	}
	sort.Strings(operations)
	for _, name := range operations {
		item := stats[operation(name)]
		fmt.Printf("operation=%s total=%d success=%d 429=%d 4xx=%d 5xx=%d errors=%d p95=%s p99=%s\n",
			name, item.total, item.success, item.limited, item.clientErr, item.serverErr, item.errors,
			percentile(item.latencies, 0.95), percentile(item.latencies, 0.99))
	}
}

func failureRate(stats operationStats) float64 {
	if stats.total == 0 {
		return 0
	}
	return float64(stats.total-stats.success) / float64(stats.total)
}

func percentile(values []time.Duration, quantile float64) time.Duration {
	if len(values) == 0 {
		return 0
	}
	sorted := append([]time.Duration(nil), values...)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i] < sorted[j] })
	index := int(float64(len(sorted)-1) * quantile)
	return sorted[index].Round(time.Microsecond)
}
