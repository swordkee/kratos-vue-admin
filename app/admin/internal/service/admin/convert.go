package admin

import (
	pb "github.com/swordkee/kratos-vue-admin/api/admin/v1"
	"github.com/swordkee/kratos-vue-admin/app/admin/internal/data/gen/model"
	"github.com/swordkee/kratos-vue-admin/pkg/util"
)

func ConvertApiBaseFromList(list [][]string) []*pb.ApiBase {
	data := make([]*pb.ApiBase, len(list))
	for i, v := range list {
		data[i] = &pb.ApiBase{
			Path:   v[1],
			Method: v[2],
		}
	}
	return data
}

func ConvertApiDataFromApiList(list []*model.SysApis) []*pb.ApiData {
	data := make([]*pb.ApiData, len(list))
	for i, d := range list {
		data[i] = &pb.ApiData{
			Id:          int32(d.ID),
			Path:        d.Path,
			Description: d.Description,
			ApiGroup:    d.APIGroup,
			Method:      d.Method,
			CreateTime:  util.NewTimestamp(d.CreatedAt),
			UpdateTime:  util.NewTimestamp(d.UpdatedAt),
		}
	}
	return data
}
