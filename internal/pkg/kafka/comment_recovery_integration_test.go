package kafka

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jmoiron/sqlx"
	"github.com/segmentio/kafka-go"
	"github.com/stretchr/testify/require"
	"github.com/your-username/forum/internal/dao/mysql"
	"github.com/your-username/forum/internal/model"
)

// Opt-in only: stops/starts services in the dedicated forum-ci Compose project.
// Never points at the production Compose file or database.
func TestCommentRecoveryIntegration(t *testing.T) {
	if testing.Short() || os.Getenv("FORUM_CI_RECOVERY") != "1" {
		t.Skip("set FORUM_CI_RECOVERY=1 with the isolated CI Compose stack")
	}
	root, err := filepath.Abs("../../..")
	require.NoError(t, err)
	compose := func(args ...string) error {
		ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
		defer cancel()
		cmd := exec.CommandContext(ctx, "docker", append([]string{"compose", "-p", "forum-ci", "-f", filepath.Join(root, "docker/ci/compose.yml")}, args...)...)
		out, err := cmd.CombinedOutput()
		t.Logf("compose %v: %s", args, out)
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	db, err := sqlx.Connect("mysql", "root:ci_password@tcp(127.0.0.1:33306)/forum?parseTime=true&timeout=1s&readTimeout=1s&writeTimeout=1s")
	require.NoError(t, err)
	defer db.Close()
	mysql.SetDB(db)
	topic := fmt.Sprintf("comment-recovery-%d", time.Now().UnixNano())
	clientID := fmt.Sprintf("recovery-%d", time.Now().UnixNano())
	comment := model.Comment{AuthorID: 987654, PostID: 987654, Content: "accepted before database recovery", ClientCommentID: clientID}
	w := &kafka.Writer{Addr: kafka.TCP("localhost:29092"), Topic: topic, AllowAutoTopicCreation: true, RequiredAcks: kafka.RequireAll, BatchSize: 1}
	defer w.Close()
	payload, err := json.Marshal(comment)
	require.NoError(t, err)
	// Pre-create metadata so this checks durable delivery rather than topic-creation races.
	conn, err := kafka.DialContext(ctx, "tcp", "localhost:29092")
	require.NoError(t, err)
	err = conn.CreateTopics(kafka.TopicConfig{Topic: topic, NumPartitions: 1, ReplicationFactor: 1})
	conn.Close()
	require.NoError(t, err)
	require.NoError(t, w.WriteMessages(ctx, kafka.Message{Key: []byte("comment"), Value: payload}, kafka.Message{Key: []byte("comment"), Value: payload}))
	t.Log("two deliveries acknowledged for one logical comment")
	require.NoError(t, compose("stop", "mysql"))
	t.Cleanup(func() { _ = compose("up", "-d", "--wait", "--wait-timeout", "120", "mysql", "kafka") })
	reader := func() *kafka.Reader {
		return kafka.NewReader(kafka.ReaderConfig{Brokers: []string{"localhost:29092"}, Topic: topic, GroupID: topic, StartOffset: kafka.FirstOffset})
	}
	r := reader()
	firstCtx, firstCancel := context.WithCancel(ctx)
	defer firstCancel()
	failed := make(chan struct{}, 1)
	done := make(chan error, 1)
	var commits atomic.Int32
	wrapped := controlledReader{fetch: r.FetchMessage, commit: func(c context.Context, m ...kafka.Message) error { commits.Add(1); return r.CommitMessages(c, m...) }}
	go func() {
		done <- consumeReliably(firstCtx, wrapped, func(m kafka.Message) error {
			err := processMessage(m)
			if err != nil {
				select {
				case failed <- struct{}{}:
				default:
				}
				return err
			}
			// Crash window: DB committed, but Kafka offset has not been committed.
			firstCancel()
			return errors.New("simulated shutdown after DB commit")
		}, 20*time.Millisecond)
	}()
	select {
	case <-failed:
	case <-ctx.Done():
		t.Fatal("did not observe DB outage")
	}
	require.Zero(t, commits.Load(), "failed persistence must not commit an offset")
	t.Log("DB offline: processing failed and offset remained uncommitted")
	require.NoError(t, compose("up", "-d", "--wait", "--wait-timeout", "120", "mysql"))
	select {
	case <-done:
	case <-ctx.Done():
		t.Fatal("consumer did not resume after DB recovery")
	}
	require.NoError(t, r.Close())
	require.Zero(t, commits.Load(), "simulated crash must leave the message for replay")
	// Broker restart must retain the acknowledged backlog, too.
	require.NoError(t, compose("stop", "kafka"))
	require.NoError(t, compose("up", "-d", "--wait", "--wait-timeout", "120", "kafka"))
	r2 := reader()
	defer r2.Close()
	secondCtx, secondCancel := context.WithCancel(ctx)
	defer secondCancel()
	wrapped2 := controlledReader{fetch: r2.FetchMessage, commit: func(c context.Context, m ...kafka.Message) error {
		err := r2.CommitMessages(c, m...)
		if err == nil && commits.Add(1) == 2 {
			secondCancel()
		}
		return err
	}}
	err = consumeReliably(secondCtx, wrapped2, processMessage, time.Millisecond)
	require.ErrorIs(t, err, context.Canceled)
	var count int
	require.NoError(t, db.Get(&count, "SELECT COUNT(*) FROM comments c JOIN comment_submissions s ON s.comment_id=c.id WHERE s.author_id=? AND s.client_comment_id=?", comment.AuthorID, clientID))
	require.Equal(t, 1, count)
	require.EqualValues(t, 2, commits.Load())
	t.Log("DB recovered + consumer replay + broker restart: 2 deliveries committed, exactly 1 comment persisted")
}

func TestCommentDeadLetterIntegration(t *testing.T) {
	if testing.Short() || os.Getenv("FORUM_CI_RECOVERY") != "1" {
		t.Skip("requires isolated CI stack")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	db, err := sqlx.Connect("mysql", "root:ci_password@tcp(127.0.0.1:33306)/forum?parseTime=true&timeout=1s")
	require.NoError(t, err)
	defer db.Close()
	mysql.SetDB(db)
	topic := fmt.Sprintf("comment-dlq-test-%d", time.Now().UnixNano())
	conn, err := kafka.DialContext(ctx, "tcp", "localhost:29092")
	require.NoError(t, err)
	err = conn.CreateTopics(kafka.TopicConfig{Topic: topic, NumPartitions: 1, ReplicationFactor: 1}, kafka.TopicConfig{Topic: topic + "-dlq", NumPartitions: 1, ReplicationFactor: 1})
	conn.Close()
	require.NoError(t, err)
	for _, name := range []string{topic, topic + "-dlq"} {
		require.Eventually(t, func() bool {
			probeCtx, stop := context.WithTimeout(ctx, time.Second)
			defer stop()
			leader, err := kafka.DialLeader(probeCtx, "tcp", "localhost:29092", name, 0)
			if err != nil {
				return false
			}
			leader.Close()
			return true
		}, 10*time.Second, 100*time.Millisecond, "topic leader not ready: %s", name)
	}
	writer := func(topic string) *kafka.Writer {
		return &kafka.Writer{Addr: kafka.TCP("localhost:29092"), Topic: topic, RequiredAcks: kafka.RequireAll, BatchSize: 1}
	}
	source, dlq := writer(topic), writer(topic+"-dlq")
	defer source.Close()
	defer dlq.Close()
	// Same ID, different payload: the failed submission must not hide the
	// previously published comment, and the next valid event must still land.
	original := model.Comment{AuthorID: 987654, PostID: 7, ClientCommentID: topic, Content: "original"}
	require.NoError(t, mysql.CreateComment(&original))
	bad := original
	bad.Content = "conflicting submission"
	next := original
	next.ClientCommentID += "-next"
	next.Content = "later valid comment"
	publish := func(c model.Comment) {
		data, err := json.Marshal(c)
		require.NoError(t, err)
		require.NoError(t, source.WriteMessages(ctx, kafka.Message{Key: []byte("comment"), Value: data}))
	}
	publish(bad)
	publish(next)
	reader := kafka.NewReader(kafka.ReaderConfig{Brokers: []string{"localhost:29092"}, Topic: topic, GroupID: topic, StartOffset: kafka.FirstOffset})
	defer reader.Close()
	commits, sends := 0, 0
	wrapped := controlledReader{fetch: reader.FetchMessage, commit: func(c context.Context, m ...kafka.Message) error {
		require.GreaterOrEqual(t, sends, 2, "must not acknowledge a failed DLQ write")
		if err := reader.CommitMessages(c, m...); err != nil {
			return err
		}
		commits++
		if commits == 2 {
			cancel()
		}
		return nil
	}}
	process := withDeadLetter(func(c context.Context, m ...kafka.Message) error {
		sends++
		if sends == 1 {
			require.Zero(t, commits)
			return errors.New("injected DLQ outage")
		}
		return dlq.WriteMessages(c, m...)
	}, mysql.RecordCommentSubmissionFailure)
	require.ErrorIs(t, consumeReliably(ctx, wrapped, process, time.Millisecond), context.Canceled)
	require.Equal(t, 2, commits)
	require.Equal(t, 2, sends)
	existing, err := mysql.GetCommentSubmission(original.AuthorID, original.ClientCommentID)
	require.NoError(t, err)
	require.NotNil(t, existing)
	require.Equal(t, "original", existing.Content)
	failed, err := mysql.HasCommentSubmissionFailure(bad.AuthorID, bad.ClientCommentID, bad.Fingerprint())
	require.NoError(t, err)
	require.True(t, failed)
	failed, err = mysql.HasCommentSubmissionFailure(original.AuthorID, original.ClientCommentID, original.Fingerprint())
	require.NoError(t, err)
	require.False(t, failed)
	existing, err = mysql.GetCommentSubmission(next.AuthorID, next.ClientCommentID)
	require.NoError(t, err)
	require.NotNil(t, existing)
	readCtx, stop := context.WithTimeout(context.Background(), 10*time.Second)
	defer stop()
	dlqReader := kafka.NewReader(kafka.ReaderConfig{Brokers: []string{"localhost:29092"}, Topic: topic + "-dlq"})
	defer dlqReader.Close()
	msg, err := dlqReader.FetchMessage(readCtx)
	require.NoError(t, err)
	require.Contains(t, string(msg.Value), bad.Content)
	require.Equal(t, topic+"/0/0", string(msg.Key))
}
