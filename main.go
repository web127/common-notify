package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"code.byted.org/fintech_cf/common-notify/internal/api"
	"code.byted.org/fintech_cf/common-notify/internal/service"
	"code.byted.org/fintech_cf/common-notify/internal/store"
	"code.byted.org/fintech_cf/common-notify/internal/worker"
	"github.com/gin-gonic/gin"
)

func main() {
	// 初始化存储
	dbPath := "./common-notify.db"
	s, err := store.NewStore(dbPath)
	if err != nil {
		log.Fatalf("failed to create store: %v", err)
	}
	defer s.Close()

	// 初始化服务
	vendorService := service.NewVendorService(s)
	notificationService := service.NewNotificationService(s, vendorService)

	// 初始化 API
	handler := api.NewHandler(notificationService, vendorService)

	// 设置 Gin
	gin.SetMode(gin.ReleaseMode)
	r := gin.Default()
	handler.RegisterRoutes(r)

	// 初始化并启动 worker
	w := worker.NewWorker(notificationService)
	if err := w.Start(); err != nil {
		log.Fatalf("failed to start worker: %v", err)
	}
	defer w.Stop()

	// 启动 HTTP 服务器
	srv := &http.Server{
		Addr:    ":8080",
		Handler: r,
	}

	go func() {
		log.Println("server starting on :8080")
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("failed to start server: %v", err)
		}
	}()

	// 优雅关闭
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	log.Println("shutting down...")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		log.Printf("server shutdown error: %v", err)
	}

	log.Println("server exited")
}
