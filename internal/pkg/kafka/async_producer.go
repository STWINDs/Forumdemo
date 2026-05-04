package kafka

import (
	"context"
	"encoding/json"
	"os"
	"sync"
	"time"

	"github.com/segmentio/kafka-go"
	"go.uber.org/zap"
)

const (
	asyncBufferSize  = 10000
	asyncFlushSize   = 100
	asyncFlushInterval = 100 * time.Millisecond
	asyncMaxRetries  = 3
	dlqFile          = "logs/vote_dlq.json"
)

var (
	asyncProd *AsyncProducer
	asyncMu   sync.Mutex
)

type AsyncProducer struct {
	buffer  chan kafka.Message
	dlqMu   sync.Mutex
	dlqFile *os.File
	stopCh  chan struct{}
}

func InitAsyncProducer() {
	asyncMu.Lock()
	defer asyncMu.Unlock()
	if asyncProd != nil {
		return
	}
	os.MkdirAll("logs", 0755)
	f, err := os.OpenFile(dlqFile, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		zap.L().Error("cannot open DLQ file", zap.Error(err))
		f = nil
	}
	asyncProd = &AsyncProducer{
		buffer:  make(chan kafka.Message, asyncBufferSize),
		dlqFile: f,
		stopCh:  make(chan struct{}),
	}
	go asyncProd.run()
	zap.L().Info("async kafka producer started")
}

func (p *AsyncProducer) run() {
	batch := make([]kafka.Message, 0, asyncFlushSize)
	ticker := time.NewTicker(asyncFlushInterval)
	defer ticker.Stop()

	for {
		select {
		case msg := <-p.buffer:
			batch = append(batch, msg)
			if len(batch) >= asyncFlushSize {
				p.flush(batch)
				batch = batch[:0]
			}
		case <-ticker.C:
			if len(batch) > 0 {
				p.flush(batch)
				batch = batch[:0]
			}
		case <-p.stopCh:
			// Drain remaining
			for {
				select {
				case msg := <-p.buffer:
					batch = append(batch, msg)
				default:
					if len(batch) > 0 {
						p.flush(batch)
					}
					zap.L().Info("async kafka producer stopped",
						zap.Int("remaining", len(batch)))
					return
				}
			}
		}
	}
}

func (p *AsyncProducer) flush(batch []kafka.Message) {
	if writer == nil {
		p.writeDLQ(batch)
		return
	}

	var lastErr error
	for attempt := 0; attempt < asyncMaxRetries; attempt++ {
		if attempt > 0 {
			backoff := time.Duration(200<<(attempt-1)) * time.Millisecond
			time.Sleep(backoff)
			zap.L().Warn("kafka async flush retry",
				zap.Int("attempt", attempt+1),
				zap.Int("batch_size", len(batch)))
		}
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		err := writer.WriteMessages(ctx, batch...)
		cancel()
		if err == nil {
			return
		}
		lastErr = err
		zap.L().Error("kafka async flush failed", zap.Error(err), zap.Int("attempt", attempt+1))
	}

	// Retry exhausted → write to dead letter file
	zap.L().Error("kafka async flush exhausted retries, writing to DLQ",
		zap.Int("batch_size", len(batch)))
	p.writeDLQ(batch)

	// Log last error for monitoring
	if lastErr != nil {
		zap.L().Error("kafka DLQ reason", zap.Error(lastErr))
	}
}

func (p *AsyncProducer) writeDLQ(batch []kafka.Message) {
	p.dlqMu.Lock()
	defer p.dlqMu.Unlock()

	if p.dlqFile == nil {
		return
	}
	for _, msg := range batch {
		data, _ := json.Marshal(map[string]interface{}{
			"key":       string(msg.Key),
			"value":     string(msg.Value),
			"timestamp": time.Now().Unix(),
		})
		p.dlqFile.Write(append(data, '\n'))
	}
}

// EnqueueEvent 异步入队，不阻塞调用方。channel 满时丢弃（Redis 已记录，对账会修复）。
func EnqueueEvent(key string, value interface{}) {
	InitAsyncProducer()

	b, err := json.Marshal(value)
	if err != nil {
		zap.L().Error("json.Marshal failed in async producer", zap.Error(err))
		return
	}

	msg := kafka.Message{
		Key:   []byte(key),
		Value: b,
	}

	select {
	case asyncProd.buffer <- msg:
	default:
		// Channel full — this vote is already in Redis, reconciliation will catch it
		zap.L().Warn("async producer buffer full, dropping message (reconciliation will catch)",
			zap.String("key", key))
	}
}

func StopAsyncProducer() {
	if asyncProd != nil {
		select {
		case <-asyncProd.stopCh:
			// already closed
		default:
			close(asyncProd.stopCh)
		}
		if asyncProd.dlqFile != nil {
			asyncProd.dlqFile.Close()
			asyncProd.dlqFile = nil
		}
	}
	asyncProd = nil
}
