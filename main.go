// Package main common-notify 服务入口
// 外部 HTTP 通知投递系统 - 接收业务系统提交的外部 HTTP 通知请求，并可靠地投递到目标地址。
package main

import (
	"log"
	"net/http"

	"code.byted.org/fintech_cf/common-notify/internal/api"
	"code.byted.org/fintech_cf/common-notify/internal/store"
	"code.byted.org/fintech_cf/common-notify/internal/worker"
)

// main 服务启动入口
// 初始化流程：
// 1. 创建 SQLite 存储层
// 2. 创建并启动 Worker 池
// 3. 创建 HTTP Handler
// 4. 注册路由
// 5. 启动 HTTP 服务
func main() {
	// 1. 初始化 SQLite 存储层，数据库文件为 ./notifications.db
	s, err := store.NewSQLiteStore("./notifications.db")
	if err != nil {
		log.Fatalf("failed to open store: %v", err)
	}
	defer s.Close()

	// 2. 创建 Worker：3 个并发 worker，队列大小 1000
	w := worker.NewWorker(s, 3, 1000)
	w.Start()

	// 3. 创建 HTTP Handler
	h := api.NewHandler(s, w)

	// 4. 注册 HTTP 路由
	mux := http.NewServeMux()
	h.RegisterRoutes(mux)

	// 5. 启动 HTTP 服务，监听 8080 端口
	log.Println("server starting on :8080")
	if err := http.ListenAndServe(":8080", mux); err != nil {
		log.Fatalf("server failed: %v", err)
	}
}
