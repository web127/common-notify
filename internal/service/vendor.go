package service

import (
	"code.byted.org/fintech_cf/common-notify/internal/model"
	"code.byted.org/fintech_cf/common-notify/internal/store"
)

// VendorService 供应商服务
type VendorService struct {
	store *store.Store
}

// NewVendorService 创建供应商服务
func NewVendorService(s *store.Store) *VendorService {
	return &VendorService{store: s}
}

// CreateVendor 创建供应商
func (s *VendorService) CreateVendor(vendor *model.Vendor) error {
	// 设置默认值
	if vendor.Method == "" {
		vendor.Method = "POST"
	}
	if vendor.TimeoutMs == 0 {
		vendor.TimeoutMs = 5000
	}
	return s.store.CreateVendor(vendor)
}

// GetVendor 获取供应商
func (s *VendorService) GetVendor(id string) (*model.Vendor, error) {
	return s.store.GetVendor(id)
}
