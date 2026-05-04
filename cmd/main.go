package main

import (
	"fmt"
	"github.com/your-username/forum/config"
	"github.com/your-username/forum/internal/dao/mysql"
	"github.com/your-username/forum/internal/dao/redis"
	"github.com/your-username/forum/internal/pkg/kafka"
	"github.com/your-username/forum/internal/pkg/logger"
	"github.com/your-username/forum/internal/pkg/minio"
	"github.com/your-username/forum/internal/router"
	"go.uber.org/zap"
)

func main() {
	// 1. Load config
	if err := config.Init(); err != nil {
		fmt.Printf("init config failed, err:%v\n", err)
		return
	}

	// 2. Init logger
	if err := logger.Init(config.Conf.App.Mode); err != nil {
		fmt.Printf("init logger failed, err:%v\n", err)
		return
	}
	defer zap.L().Sync()

	// 3. Init MySQL
	if err := mysql.Init(config.Conf.MySQL); err != nil {
		zap.L().Warn("mysql init failed, API will fallback to errors", zap.Error(err))
	} else {
		defer mysql.Close()
	}

	// 4. Init Redis
	if err := redis.Init(config.Conf.Redis); err != nil {
		zap.L().Warn("redis init failed, cache disabled", zap.Error(err))
	} else {
		defer redis.Close()
	}

	// 5. Init Minio
	if err := minio.Init(config.Conf.Minio); err != nil {
		zap.L().Warn("minio init failed, video upload disabled", zap.Error(err))
	}

	// 6. Init Kafka + Async Producer + Reconciler
	kafka.Init(config.Conf.Kafka)
	kafka.InitConsumer(config.Conf.Kafka)
	kafka.InitCanalConsumer(config.Conf.Kafka)
	kafka.InitAsyncProducer()
	kafka.StartVoteReconciliationLoop()
	defer kafka.Close()

	// 7. Register routes (serves frontend + API)
	r := router.Setup(config.Conf.App.Mode)

	// 8. Start server
	zap.L().Info("Starting server", zap.Int("port", config.Conf.App.Port))
	err := r.Run(fmt.Sprintf(":%d", config.Conf.App.Port))
	if err != nil {
		fmt.Printf("run server failed, err:%v\n", err)
		return
	}
}
