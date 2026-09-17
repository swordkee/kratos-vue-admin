package sms

import (
	"context"
	"fmt"

	"gorm.io/gorm"
)

// GatewayRepo 短信网关表数据访问（sys_sms_gateway）
type GatewayRepo interface {
	FindByFeatureID(ctx context.Context, featureID int32) (*SysSmsGateway, error)
	FindByType(ctx context.Context, smsType string) (*SysSmsGateway, error)
	FindAllEnabled(ctx context.Context) ([]*SysSmsGateway, error)
}

// GatewayRepoImpl gorm 实现
type GatewayRepoImpl struct {
	db *gorm.DB
}

// NewGatewayRepo 创建仓库
func NewGatewayRepo(db *gorm.DB) GatewayRepo {
	return &GatewayRepoImpl{db: db}
}

func (r *GatewayRepoImpl) FindByFeatureID(ctx context.Context, featureID int32) (*SysSmsGateway, error) {
	var gateway SysSmsGateway
	err := r.db.WithContext(ctx).
		Where("feature_id = ? AND status = 1", featureID).
		First(&gateway).Error
	if err != nil {
		return nil, fmt.Errorf("查询功能 ID 为 %d 的短信网关配置失败: %w", featureID, err)
	}
	return &gateway, nil
}

func (r *GatewayRepoImpl) FindByType(ctx context.Context, smsType string) (*SysSmsGateway, error) {
	var gateway SysSmsGateway
	err := r.db.WithContext(ctx).
		Where("sms_type = ? AND status = 1", smsType).
		First(&gateway).Error
	if err != nil {
		return nil, fmt.Errorf("查询类型为 %s 的短信网关配置失败: %w", smsType, err)
	}
	return &gateway, nil
}

func (r *GatewayRepoImpl) FindAllEnabled(ctx context.Context) ([]*SysSmsGateway, error) {
	var gateways []*SysSmsGateway
	err := r.db.WithContext(ctx).Where("status = 1").Order("feature_id ASC, id ASC").Find(&gateways).Error
	if err != nil {
		return nil, fmt.Errorf("查询短信网关配置失败: %w", err)
	}
	return gateways, nil
}
