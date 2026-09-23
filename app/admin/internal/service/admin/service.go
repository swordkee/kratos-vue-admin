package admin

import (
	"github.com/google/wire"
)

// ProviderSet is service providers.
var ProviderSet = wire.NewSet(
	NewSysUserService,
	NewSysLogsService,
	NewMenusService,
	NewRolesService,
	NewApiService,
	NewDeptService,
	NewPostService,
	NewDictDataService,
	NewDictTypeService,
	// 统一站内信（TASK-03 批1 模板版）：与 service.ProviderSet 镜像登记
	NewMessageService,
)
