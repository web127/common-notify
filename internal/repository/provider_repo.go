package repository

import (
	"database/sql"
	"time"

	"code.byted.org/fintech_cf/common-notify/internal/model"
)

// ProviderRepository 供应商仓库
type ProviderRepository struct {
	db *DB
}

// NewProviderRepository 创建供应商仓库
func NewProviderRepository(db *DB) *ProviderRepository {
	return &ProviderRepository{db: db}
}

// Create 创建供应商
func (r *ProviderRepository) Create(p *model.Provider) error {
	now := time.Now()
	p.CreatedAt = now
	p.UpdatedAt = now

	result, err := r.db.Exec(`
		INSERT INTO providers (name, base_url, method, headers, body_tpl, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)
	`, p.Name, p.BaseURL, p.Method, p.Headers, p.BodyTpl, p.CreatedAt, p.UpdatedAt)
	if err != nil {
		return err
	}

	id, err := result.LastInsertId()
	if err != nil {
		return err
	}
	p.ID = id
	return nil
}

// GetByID 根据 ID 获取供应商
func (r *ProviderRepository) GetByID(id int64) (*model.Provider, error) {
	var p model.Provider
	err := r.db.QueryRow(`
		SELECT id, name, base_url, method, headers, body_tpl, created_at, updated_at
		FROM providers WHERE id = ?
	`, id).Scan(&p.ID, &p.Name, &p.BaseURL, &p.Method, &p.Headers, &p.BodyTpl, &p.CreatedAt, &p.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &p, nil
}

// GetByName 根据名称获取供应商
func (r *ProviderRepository) GetByName(name string) (*model.Provider, error) {
	var p model.Provider
	err := r.db.QueryRow(`
		SELECT id, name, base_url, method, headers, body_tpl, created_at, updated_at
		FROM providers WHERE name = ?
	`, name).Scan(&p.ID, &p.Name, &p.BaseURL, &p.Method, &p.Headers, &p.BodyTpl, &p.CreatedAt, &p.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &p, nil
}

// List 获取所有供应商
func (r *ProviderRepository) List() ([]*model.Provider, error) {
	rows, err := r.db.Query(`
		SELECT id, name, base_url, method, headers, body_tpl, created_at, updated_at
		FROM providers ORDER BY id
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var providers []*model.Provider
	for rows.Next() {
		var p model.Provider
		if err := rows.Scan(&p.ID, &p.Name, &p.BaseURL, &p.Method, &p.Headers, &p.BodyTpl, &p.CreatedAt, &p.UpdatedAt); err != nil {
			return nil, err
		}
		providers = append(providers, &p)
	}
	return providers, rows.Err()
}

// Update 更新供应商
func (r *ProviderRepository) Update(p *model.Provider) error {
	p.UpdatedAt = time.Now()
	_, err := r.db.Exec(`
		UPDATE providers
		SET name = ?, base_url = ?, method = ?, headers = ?, body_tpl = ?, updated_at = ?
		WHERE id = ?
	`, p.Name, p.BaseURL, p.Method, p.Headers, p.BodyTpl, p.UpdatedAt, p.ID)
	return err
}

// Delete 删除供应商
func (r *ProviderRepository) Delete(id int64) error {
	_, err := r.db.Exec("DELETE FROM providers WHERE id = ?", id)
	return err
}
