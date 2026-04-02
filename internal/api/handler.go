package api

import (
	"net/http"
	"strconv"

	"code.byted.org/fintech_cf/common-notify/internal/model"
	"code.byted.org/fintech_cf/common-notify/internal/service"
	"github.com/gin-gonic/gin"
)

// Handler API 处理器
type Handler struct {
	notificationService *service.NotificationService
	vendorService       *service.VendorService
}

// NewHandler 创建 API 处理器
func NewHandler(ns *service.NotificationService, vs *service.VendorService) *Handler {
	return &Handler{
		notificationService: ns,
		vendorService:       vs,
	}
}

// RegisterRoutes 注册路由
func (h *Handler) RegisterRoutes(r *gin.Engine) {
	// 供应商管理
	r.POST("/vendors", h.CreateVendor)
	r.GET("/vendors/:id", h.GetVendor)

	// 通知管理
	r.POST("/notifications", h.CreateNotification)
	r.GET("/notifications/:id", h.GetNotification)
	r.POST("/notifications/:id/retry", h.RetryNotification)
}

// CreateVendor 创建供应商
func (h *Handler) CreateVendor(c *gin.Context) {
	var vendor model.Vendor
	if err := c.ShouldBindJSON(&vendor); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if vendor.ID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "vendor id is required"})
		return
	}

	if err := h.vendorService.CreateVendor(&vendor); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, vendor)
}

// GetVendor 获取供应商
func (h *Handler) GetVendor(c *gin.Context) {
	id := c.Param("id")

	vendor, err := h.vendorService.GetVendor(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "vendor not found"})
		return
	}

	c.JSON(http.StatusOK, vendor)
}

// CreateNotification 创建通知
func (h *Handler) CreateNotification(c *gin.Context) {
	var req struct {
		OutBizNo  string                 `json:"out_biz_no" binding:"required"`
		VendorID  string                 `json:"vendor_id" binding:"required"`
		EventType string                 `json:"event_type" binding:"required"`
		Payload   map[string]interface{} `json:"payload" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	notification := &model.Notification{
		OutBizNo:  req.OutBizNo,
		VendorID:  req.VendorID,
		EventType: req.EventType,
		Payload:   req.Payload,
	}

	created, err := h.notificationService.CreateNotification(notification)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, created)
}

// GetNotification 获取通知
func (h *Handler) GetNotification(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid notification id"})
		return
	}

	notification, err := h.notificationService.GetNotification(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "notification not found"})
		return
	}

	c.JSON(http.StatusOK, notification)
}

// RetryNotification 重试通知
func (h *Handler) RetryNotification(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid notification id"})
		return
	}

	if err := h.notificationService.RetryNotification(id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "notification queued for retry"})
}
