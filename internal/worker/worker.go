package worker

import (
	"log"

	"code.byted.org/fintech_cf/common-notify/internal/service"
	"github.com/robfig/cron/v3"
)

// Worker 定时任务 worker
type Worker struct {
	cron                *cron.Cron
	notificationService *service.NotificationService
}

// NewWorker 创建 worker
func NewWorker(ns *service.NotificationService) *Worker {
	return &Worker{
		cron:                cron.New(cron.WithSeconds()),
		notificationService: ns,
	}
}

// Start 启动 worker
func (w *Worker) Start() error {
	// 每 10 秒执行一次
	_, err := w.cron.AddFunc("*/10 * * * * *", func() {
		if err := w.notificationService.ProcessPendingNotifications(100); err != nil {
			log.Printf("error processing notifications: %v", err)
		}
	})

	if err != nil {
		return err
	}

	w.cron.Start()
	log.Println("worker started")
	return nil
}

// Stop 停止 worker
func (w *Worker) Stop() {
	ctx := w.cron.Stop()
	<-ctx.Done()
	log.Println("worker stopped")
}
