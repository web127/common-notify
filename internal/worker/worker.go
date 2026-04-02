// Package worker 提供通知投递的 worker 实现
// 负责从存储中获取任务并投递到外部系统，包含重试和死信处理
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

// Worker 通知投递工作器
// 管理 worker 池、任务队列和 HTTP 客户端
type Worker struct {
	store     *store.SQLiteStore       // 存储层
	queue     chan *model.Notification // 任务队列
	workerNum int                      // worker 数量
	client    *http.Client             // HTTP 客户端
}

// NewWorker 创建一个新的 Worker 实例
// store: 存储层实例
// workerNum: worker 协程数量
// queueSize: 任务队列大小
func NewWorker(store *store.SQLiteStore, workerNum int, queueSize int) *Worker {
	return &Worker{
		store:     store,
		queue:     make(chan *model.Notification, queueSize),
		workerNum: workerNum,
		client: &http.Client{
			Timeout: 30 * time.Second, // HTTP 请求超时 30 秒
		},
	}
}

// Start 启动 worker 池
// 启动指定数量的 worker 协程，并启动任务加载和定时轮询
func (w *Worker) Start() {
	// 启动 worker 协程
	for i := 0; i < w.workerNum; i++ {
		go w.work()
	}
	// 服务启动时加载 pending 任务
	go w.loadPendingTasks()
	// 定时轮询 DB 获取新的 pending 任务
	go w.pollPendingTasks()
}

// Submit 提交一个通知任务到队列
// n: 通知任务对象
func (w *Worker) Submit(n *model.Notification) {
	select {
	case w.queue <- n:
		// 任务成功入队
	default:
		// 队列已满，记录日志并丢弃（任务仍在 DB 中，会被轮询重新加载）
		log.Printf("queue full, dropped notification: %d", n.ID)
	}
}

// loadPendingTasks 服务启动时加载所有 pending 任务
// 从 DB 读取 pending 任务并提交到队列
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

// pollPendingTasks 定时轮询 DB 获取 pending 任务
// 每 5 秒轮询一次，获取可执行的 pending 任务
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

// work worker 主循环
// 从队列中获取任务并处理
func (w *Worker) work() {
	for n := range w.queue {
		w.processNotification(n)
	}
}

// processNotification 处理单个通知任务
// 1. 标记为 processing
// 2. 发送 HTTP 请求
// 3. 成功则标记为 success
// 4. 失败则安排重试或标记为 dead
func (w *Worker) processNotification(n *model.Notification) {
	// 先更新状态为 processing，避免重复处理
	if err := w.store.UpdateStatus(n.ID, model.StatusProcessing); err != nil {
		log.Printf("failed to update status to processing: %v", err)
		return
	}

	// 发送 HTTP 请求
	err := w.sendHTTPRequest(n)
	if err == nil {
		// 成功，标记为 success
		if err := w.store.MarkSuccess(n.ID); err != nil {
			log.Printf("failed to mark success: %v", err)
		}
		return
	}

	// 失败，计算重试次数
	retryCount := n.RetryCount + 1
	if retryCount >= n.MaxRetries {
		// 超过最大重试次数，标记为 dead（死信）
		if err := w.store.MarkDead(n.ID, err.Error()); err != nil {
			log.Printf("failed to mark dead: %v", err)
		}
		return
	}

	// 安排下次重试（指数退避）
	nextRetryAt := time.Now().Add(calculateBackoff(retryCount))
	if err := w.store.ScheduleRetry(n.ID, retryCount, nextRetryAt, err.Error()); err != nil {
		log.Printf("failed to schedule retry: %v", err)
	}
}

// sendHTTPRequest 发送 HTTP 请求到外部系统
// n: 通知任务对象
// 返回：成功返回 nil，失败返回 error
func (w *Worker) sendHTTPRequest(n *model.Notification) error {
	// 解析 Headers（JSON 字符串 -> map）
	var headers map[string]string
	if n.Headers != "" {
		json.Unmarshal([]byte(n.Headers), &headers)
	}

	// 创建 HTTP 请求
	req, err := http.NewRequest(n.Method, n.TargetURL, bytes.NewReader([]byte(n.Body)))
	if err != nil {
		return err
	}

	// 设置请求头
	for k, v := range headers {
		req.Header.Set(k, v)
	}

	// 发送请求
	resp, err := w.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	// 2xx 状态码视为成功
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return nil
	}

	return fmt.Errorf("HTTP status %d", resp.StatusCode)
}

// calculateBackoff 计算指数退避重试间隔
// retryCount: 当前重试次数（第 1 次重试返回 2s，第 2 次返回 4s...）
// 返回：重试间隔时间
func calculateBackoff(retryCount int) time.Duration {
	// 指数退避：1s, 2s, 4s, 8s, 16s...
	return time.Duration(1<<retryCount) * time.Second
}
