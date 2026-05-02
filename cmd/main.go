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
		fmt.Printf("init mysql failed, err:%v\n", err)
		return
	}
	defer mysql.Close()

	// 4. Init Redis
	if err := redis.Init(config.Conf.Redis); err != nil {
		fmt.Printf("init redis failed, err:%v\n", err)
		return
	}
	defer redis.Close()

	// 5. Init Minio
	if err := minio.Init(config.Conf.Minio); err != nil {
		fmt.Printf("init minio failed, err:%v\n", err)
		return
	}

	// 6. Init Kafka
	kafka.Init(config.Conf.Kafka)
	kafka.InitConsumer(config.Conf.Kafka)
	kafka.InitCanalConsumer(config.Conf.Kafka)
	defer kafka.Close()

	// 6. Register routes
	r := router.Setup(config.Conf.App.Mode)

	// 6. Start server
	err := r.Run(fmt.Sprintf(":%d", config.Conf.App.Port))
	if err != nil {
		fmt.Printf("run server failed, err:%v\n", err)
		return
	}
}
