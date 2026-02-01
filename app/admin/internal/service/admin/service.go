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
)
