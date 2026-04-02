package worker

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	"code.byted.org/fintech_cf/common-notify/internal/model"
	"code.byted.org/fintech_cf/common-notify/internal/store"
)

type Worker struct {
	store     *store.SQLiteStore
	queue     chan *model.Notification
	workerNum int
	client    *http.Client
}

func NewWorker(store *store.SQLiteStore, workerNum int, queueSize int) *Worker {
	return &Worker{
		store:     store,
		queue:     make(chan *model.Notification, queueSize),
		workerNum: workerNum,
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

func (w *Worker) Start() {
	for i := 0; i < w.workerNum; i++ {
		go w.work()
	}
	go w.loadPendingTasks()
	go w.pollPendingTasks()
}

func (w *Worker) Submit(n *model.Notification) {
	select {
	case w.queue <- n:
	default:
		log.Printf("queue full, dropped notification: %d", n.ID)
	}
}

func (w *Worker) loadPendingTasks() {
	notifications, err := w.store.ListPendingNotifications(100)
	if err != nil {
		log.Printf("failed to load pending notifications: %v", err)
		return
	}
	for _, n := range notifications {
		w.Submit(n)
	}
}

func (w *Worker) pollPendingTasks() {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		notifications, err := w.store.ListPendingNotifications(100)
		if err != nil {
			log.Printf("failed to poll pending notifications: %v", err)
			continue
		}
		for _, n := range notifications {
			w.Submit(n)
		}
	}
}

func (w *Worker) work() {
	for n := range w.queue {
		w.processNotification(n)
	}
}

func (w *Worker) processNotification(n *model.Notification) {
	if err := w.store.UpdateStatus(n.ID, model.StatusProcessing); err != nil {
		log.Printf("failed to update status to processing: %v", err)
		return
	}

	err := w.sendHTTPRequest(n)
	if err == nil {
		if err := w.store.MarkSuccess(n.ID); err != nil {
			log.Printf("failed to mark success: %v", err)
		}
		return
	}

	retryCount := n.RetryCount + 1
	if retryCount >= n.MaxRetries {
		if err := w.store.MarkDead(n.ID, err.Error()); err != nil {
			log.Printf("failed to mark dead: %v", err)
		}
		return
	}

	nextRetryAt := time.Now().Add(calculateBackoff(retryCount))
	if err := w.store.ScheduleRetry(n.ID, retryCount, nextRetryAt, err.Error()); err != nil {
		log.Printf("failed to schedule retry: %v", err)
	}
}

func (w *Worker) sendHTTPRequest(n *model.Notification) error {
	var headers map[string]string
	if n.Headers != "" {
		json.Unmarshal([]byte(n.Headers), &headers)
	}

	req, err := http.NewRequest(n.Method, n.TargetURL, bytes.NewReader([]byte(n.Body)))
	if err != nil {
		return err
	}

	for k, v := range headers {
		req.Header.Set(k, v)
	}

	resp, err := w.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return nil
	}

	return fmt.Errorf("HTTP status %d", resp.StatusCode)
}

func calculateBackoff(retryCount int) time.Duration {
	return time.Duration(1<<retryCount) * time.Second
}
