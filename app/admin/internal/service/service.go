package service

import (
	"github.com/google/wire"
	"github.com/swordkee/kratos-vue-admin/app/admin/internal/service/admin"
)

// ProviderSet is service providers.
var ProviderSet = wire.NewSet(
	// admin模块的服务
	admin.NewSysuserService,
	admin.NewSysLogsService,
	admin.NewMenusService,
	admin.NewRolesService,
	admin.NewApiService,
	admin.NewDeptService,
	admin.NewPostService,
	admin.NewDictDataService,
	admin.NewDictTypeService,
)
