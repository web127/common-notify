# common-notify

外部 HTTP 通知投递系统 - 接收业务系统提交的外部 HTTP 通知请求，并可靠地投递到目标地址。

## 架构

- HTTP API 层：接收通知请求，幂等去重
- SQLite 存储：持久化通知任务
- Worker 池：并发投递，指数退避重试

## 快速开始

```bash
# 构建
go build -o common-notify .

# 运行
./common-notify

# 发送测试请求
curl -X POST http://localhost:8080/api/v1/notifications \
  -H "Content-Type: application/json" \
  -d '{
    "out_biz_no": "test-001",
    "event_type": "user.registered",
    "target_url": "https://httpbin.org/post",
    "method": "POST",
    "headers": {"Content-Type": "application/json"},
    "body": "{\"user_id\": 123}"
  }'
```

## 设计文档

详见 [docs/superpowers/specs/2026-04-02-common-notify-design.md](docs/superpowers/specs/2026-04-02-common-notify-design.md)
