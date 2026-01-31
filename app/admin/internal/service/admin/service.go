package admin

import (
	"github.com/google/wire"
)

// ProviderSet is service providers.
var ProviderSet = wire.NewSet(
	NewSysuserService,
	NewSysLogsService,
	NewMenusService,
	NewRolesService,
	NewApiService,
	NewDeptService,
	NewPostService,
	NewDictDataService,
	NewDictTypeService,
)
