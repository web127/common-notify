package service

import (
	"errors"

	"code.byted.org/fintech_cf/common-notify/internal/model"
	"code.byted.org/fintech_cf/common-notify/internal/repository"
)

// ProviderService 供应商服务
type ProviderService struct {
	providerRepo *repository.ProviderRepository
}

// NewProviderService 创建供应商服务
func NewProviderService(providerRepo *repository.ProviderRepository) *ProviderService {
	return &ProviderService{providerRepo: providerRepo}
}

// CreateProvider 创建供应商
func (s *ProviderService) CreateProvider(p *model.Provider) error {
	// 验证必填字段
	if p.Name == "" {
		return errors.New("name is required")
	}
	if p.BaseURL == "" {
		return errors.New("base_url is required")
	}
	if p.Method == "" {
		p.Method = "POST"
	}
	if p.Headers == nil {
		p.Headers = make(model.StringMap)
	}

	// 检查名称是否已存在
	existing, err := s.providerRepo.GetByName(p.Name)
	if err != nil {
		return err
	}
	if existing != nil {
		return errors.New("provider name already exists")
	}

	return s.providerRepo.Create(p)
}

// GetProvider 获取供应商
func (s *ProviderService) GetProvider(id int64) (*model.Provider, error) {
	return s.providerRepo.GetByID(id)
}

// ListProviders 获取所有供应商
func (s *ProviderService) ListProviders() ([]*model.Provider, error) {
	return s.providerRepo.List()
}

// UpdateProvider 更新供应商
func (s *ProviderService) UpdateProvider(p *model.Provider) error {
	if p.ID == 0 {
		return errors.New("id is required")
	}
	return s.providerRepo.Update(p)
}

// DeleteProvider 删除供应商
func (s *ProviderService) DeleteProvider(id int64) error {
	return s.providerRepo.Delete(id)
}
