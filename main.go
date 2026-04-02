package main

import (
	"flag"
	"log"
	"net/http"
	"time"

	"code.byted.org/fintech_cf/common-notify/internal/api"
	"code.byted.org/fintech_cf/common-notify/internal/repository"
	"code.byted.org/fintech_cf/common-notify/internal/service"
	"code.byted.org/fintech_cf/common-notify/internal/worker"
	"github.com/gorilla/mux"
)

func main() {
	// 命令行参数
	dsn := flag.String("db", "common_notify.db", "SQLite database DSN")
	port := flag.String("port", ":8080", "HTTP server port")
	workerInterval := flag.Int("worker-interval", 5, "Worker interval in seconds")
	batchSize := flag.Int("batch-size", 10, "Batch size for processing notifications")
	flag.Parse()

	// 初始化数据库
	db, err := repository.NewDB(*dsn)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer db.Close()

	// 初始化仓库
	providerRepo := repository.NewProviderRepository(db)
	notificationRepo := repository.NewNotificationRepository(db)

	// 初始化服务
	providerService := service.NewProviderService(providerRepo)
	notificationService := service.NewNotificationService(notificationRepo, providerRepo)

	// 初始化 HTTP 处理器
	apiHandler := api.NewHandler(providerService, notificationService)
	router := mux.NewRouter()
	apiHandler.RegisterRoutes(router)

	// 启动 Worker
	w := worker.NewWorker(notificationService,
		time.Duration(*workerInterval)*time.Second,
		*batchSize)
	w.Start()
	defer w.Stop()

	// 启动 HTTP 服务器
	log.Printf("Server starting on port %s", *port)
	log.Printf("Database: %s", *dsn)
	if err := http.ListenAndServe(*port, router); err != nil {
		log.Fatalf("Server failed: %v", err)
	}
}
