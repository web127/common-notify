package worker

import (
	"log"
	"time"

	"code.byted.org/fintech_cf/common-notify/internal/service"
)

// Worker 后台工作协程
type Worker struct {
	notificationService *service.NotificationService
	interval            time.Duration
	batchSize           int
	stopChan            chan struct{}
}

// NewWorker 创建 Worker
func NewWorker(notificationService *service.NotificationService, interval time.Duration, batchSize int) *Worker {
	if interval <= 0 {
		interval = 5 * time.Second
	}
	if batchSize <= 0 {
		batchSize = 10
	}
	return &Worker{
		notificationService: notificationService,
		interval:            interval,
		batchSize:           batchSize,
		stopChan:            make(chan struct{}),
	}
}

// Start 启动 Worker
func (w *Worker) Start() {
	log.Printf("Worker starting with interval=%v, batchSize=%d", w.interval, w.batchSize)
	go w.run()
}

// Stop 停止 Worker
func (w *Worker) Stop() {
	close(w.stopChan)
}

// run 主循环
func (w *Worker) run() {
	ticker := time.NewTicker(w.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			processed := w.notificationService.ProcessPendingNotifications(w.batchSize)
			if processed > 0 {
				log.Printf("Processed %d pending notifications", processed)
			}
		case <-w.stopChan:
			log.Println("Worker stopping")
			return
		}
	}
}
