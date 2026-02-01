package admin

import (
	"context"
	"strings"

	"github.com/go-kratos/kratos/v2/log"
	kratoshttp "github.com/go-kratos/kratos/v2/transport/http"
	"github.com/swordkee/kratos-vue-admin/pkg/common/constant"

	pb "github.com/swordkee/kratos-vue-admin/api/admin/v1"
	"github.com/swordkee/kratos-vue-admin/app/admin/internal/biz/admin"
	"github.com/swordkee/kratos-vue-admin/app/admin/internal/biz"
	"github.com/swordkee/kratos-vue-admin/app/admin/internal/conf"
	"github.com/swordkee/kratos-vue-admin/app/admin/internal/data/gen/model"
	"github.com/swordkee/kratos-vue-admin/app/admin/internal/pkg/authz"
	"github.com/swordkee/kratos-vue-admin/pkg/util"
)

type SysUserService struct {
	pb.UnimplementedSysUserServer
	serverConf   *conf.Server
	userCase     *biz.SysUserUseCase
	authCase     *admin.AuthUseCase
	roleCase     *biz.SysRoleUseCase
	roleMenuCase *biz.SysRoleMenuUseCase
	postCase     *biz.SysPostUseCase
	deptCase     *biz.SysDeptUseCase
	log          *log.Helper
}

func NewSysUserService(serverConf *conf.Server, userCase *admin.SysUserUseCase, authCase *admin.AuthUseCase, roleCase *admin.SysRoleUseCase, roleMenuCase *admin.SysRoleMenuUseCase, postCase *admin.SysPostUseCase, deptCase *admin.SysDeptUseCase, logger log.Logger) *SysUserService {
	return &SysUserService{
		serverConf:   serverConf,
		userCase:     userCase,
		authCase:     authCase,
		roleCase:     roleCase,
		roleMenuCase: roleMenuCase,
		postCase:     postCase,
		deptCase:     deptCase,
		log:          log.NewHelper(log.With(logger, "module", "service/SysUser")),
	}
}

func (s *SysUserService) CreateSysUser(ctx context.Context, req *pb.CreateSysUserRequest) (*pb.CreateSysUserReply, error) {
	if err := req.Validate(); err != nil {
		return nil, err
	}

	_, err := s.userCase.CreateSysUser(ctx, &model.SysUsers{
		NickName: req.NickName,
		Phone:    req.Phone,
		RoleID:   req.RoleId,
		Avatar:   req.Avatar,
		Sex:      req.Sex,
		Email:    req.Email,
		DeptID:   req.DeptId,
		PostID:   req.PostId,
		Remark:   req.Remark,
		Status:   req.Status,
		Username: req.Username,
		Password: req.Password,
		RoleIds:  req.RoleIds,
		PostIds:  req.PostIds,
		Secret:   req.Secret,
	})
	if err != nil {
		return nil, err
	}
	return &pb.CreateSysUserReply{}, nil
}

func (s *SysUserService) UpdateSysUser(ctx context.Context, req *pb.UpdateSysUserRequest) (*pb.UpdateSysUserReply, error) {
	if err := req.Validate(); err != nil {
		return nil, err
	}

	err := s.userCase.UpdateSysUser(ctx, &model.SysUsers{
		ID:       req.UserId,
		NickName: req.NickName,
		Phone:    req.Phone,
		RoleID:   req.RoleId,
		Avatar:   req.Avatar,
		Sex:      req.Sex,
		Email:    req.Email,
		DeptID:   req.DeptId,
		PostID:   req.PostId,
		Remark:   req.Remark,
		Status:   req.Status,
		Username: req.Username,
		Password: req.Password,
		RoleIds:  req.RoleIds,
		PostIds:  req.PostIds,
		Secret:   req.Secret,
	})
	if err != nil {
		return nil, err
	}
	return &pb.UpdateSysUserReply{}, nil
}

func (s *SysUserService) DeleteSysUser(ctx context.Context, req *pb.DeleteSysUserRequest) (*pb.DeleteSysUserReply, error) {
	err := s.userCase.DeleteSysUser(ctx, req.Id)
	return &pb.DeleteSysUserReply{}, err
}

func (s *SysUserService) FindSysUser(ctx context.Context, req *pb.FindSysUserRequest) (*pb.FindSysUserReply, error) {
	if err := req.Validate(); err != nil {
		return nil, err
	}

	user, err := s.userCase.FindSysUserById(ctx, req.Id)
	if err != nil {
		return nil, err
	}
	role, err := s.roleCase.FindRole(ctx, user.RoleID)
	if err != nil {
		return nil, err
	}

	roles, err := s.roleCase.FindRoleByIDList(ctx, util.Split2Int64Slice(user.RoleIds))
	if err != nil {
		return nil, err
	}
	posts, err := s.postCase.FindPostByIDList(ctx, util.Split2Int64Slice(user.PostIds))
	if err != nil {
		return nil, err
	}

	deptList, err := s.deptCase.QueryDeptList(ctx)
	if err != nil {
		return nil, err
	}

	replyRole := make([]*pb.RoleData, len(roles))
	for i, d := range roles {
		replyRole[i] = &pb.RoleData{
			RoleId:     d.ID,
			RoleName:   d.RoleName,
			Status:     d.Status,
			RoleKey:    d.RoleKey,
			RoleSort:   d.RoleSort,
			DataScope:  int64(d.DataScope),
			CreateBy:   d.CreateBy,
			UpdateBy:   d.UpdateBy,
			Remark:     d.Remark,
			CreateTime: util.NewTimestamp(d.CreatedAt),
			UpdateTime: util.NewTimestamp(d.UpdatedAt),
		}
	}

	replyPost := make([]*pb.PostData, len(posts))
	for i, d := range posts {
		replyPost[i] = &pb.PostData{
			PostId:     d.ID,
			PostName:   d.PostName,
			PostCode:   d.PostCode,
			Sort:       d.Sort,
			Status:     d.Status,
			Remark:     d.Remark,
			CreateBy:   d.CreateBy,
			UpdateBy:   d.UpdateBy,
			CreateTime: util.NewTimestamp(d.CreatedAt),
			UpdateTime: util.NewTimestamp(d.UpdatedAt),
		}
	}

	// 没有设置角色会报错
	roleName := ""
	if role != nil && role.RoleName != "" {
		roleName = role.RoleName
	}
	replyUser := &pb.UserData{
		UserId:     user.ID,
		NickName:   user.NickName,
		Phone:      user.Phone,
		RoleId:     int32(user.RoleID),
		Avatar:     user.Avatar,
		Sex:        int64(user.Sex),
		Email:      user.Email,
		DeptId:     int32(user.DeptID),
		PostId:     int32(user.PostID),
		RoleIds:    user.RoleIds,
		PostIds:    user.PostIds,
		CreateBy:   user.CreateBy,
		UpdateBy:   user.UpdateBy,
		Remark:     user.Remark,
		Status:     user.Status,
		Username:   roleName,
		RoleName:   role.RoleName,
		CreateTime: util.NewTimestamp(user.CreatedAt),
		UpdateTime: util.NewTimestamp(user.UpdatedAt),
		Secret:     user.Secret,
		Qrcode:     util.NewGoogleAuth().GetQrcode(user.Secret),
	}

	replyDepts := admin.ConvertToDeptTreeChildren(deptList)
	reply := &pb.FindSysUserReply{
		User:    replyUser,
		Roles:   replyRole,
		Posts:   replyPost,
		Depts:   replyDepts,
		PostIds: replyUser.PostIds,
		RoleIds: replyUser.RoleIds,
	}
	return reply, nil
}

func (s *SysUserService) ListSysUser(ctx context.Context, req *pb.ListSysUserRequest) (*pb.ListSysUserReply, error) {
	if err := req.Validate(); err != nil {
		return nil, err
	}
	users, total, err := s.userCase.ListPage(ctx, req)
	if err != nil {
		return nil, err
	}

	deptCache := util.NewCache(func(id int64) (*model.SysDepts, error) {
		d, err := s.deptCase.FindDept(ctx, id)
		if d == nil {
			d = &model.SysDepts{}
		}
		return d, err
	})
	roleCache := util.NewCache(func(id int64) (*model.SysRoles, error) {
		d, err := s.roleCase.FindRole(ctx, id)
		if d == nil {
			d = &model.SysRoles{}
		}
		return d, err
	})

	gAuth := util.NewGoogleAuth()
	replyData := make([]*pb.UserData, len(users))
	for i, user := range users {
		role, _ := roleCache.Get(user.RoleID)
		dept, _ := deptCache.Get(user.DeptID)
		replyData[i] = &pb.UserData{
			UserId:     user.ID,
			NickName:   user.NickName,
			Phone:      user.Phone,
			RoleId:     int32(user.RoleID),
			Avatar:     user.Avatar,
			Sex:        int64(user.Sex),
			Email:      user.Email,
			DeptId:     int32(user.DeptID),
			PostId:     int32(user.PostID),
			RoleIds:    user.RoleIds,
			PostIds:    user.PostIds,
			CreateBy:   user.CreateBy,
			UpdateBy:   user.UpdateBy,
			Remark:     user.Remark,
			Status:     user.Status,
			CreateTime: util.NewTimestamp(user.CreatedAt),
			UpdateTime: util.NewTimestamp(user.UpdatedAt),
			Username:   user.Username,
			RoleName:   role.RoleName,
			DeptName:   dept.DeptName,
			Secret:     user.Secret,
			Qrcode:     gAuth.GetQrcode(user.Secret),
		}
	}

	return &pb.ListSysUserReply{
		Total:    total,
		PageNum:  req.PageNum,
		PageSize: req.PageSize,
		Data:     replyData,
	}, nil
}

func (s *SysUserService) GetCaptcha(context.Context, *pb.FindCaptchaRequest) (*pb.FindCaptchaReply, error) {
	id, content, image := util.Generate()
	if s.serverConf.GetEnv() != conf.Env_dev {
		content = ""
	}
	return &pb.FindCaptchaReply{
		Base64Captcha: image,
		CaptchaId:     id,
		Content:       content,
	}, nil
}

func (s *SysUserService) Login(ctx context.Context, req *pb.LoginRequest) (*pb.LoginReply, error) {
	if err := req.Validate(); err != nil {
		return nil, err
	}

	token, expireAt, err := s.authCase.Login(ctx, req)
	if err != nil {
		return nil, err
	}

	return &pb.LoginReply{
		Token:  token,
		Expire: expireAt,
	}, nil
}

func (s *SysUserService) Logout(ctx context.Context, req *pb.LogoutRequest) (*pb.LogoutReply, error) {
	// 从 context 获取 JWT token
	rawToken := ""

	// 从 transport 获取请求信息
	if httpReq, ok := kratoshttp.RequestFromServerContext(ctx); ok {
		authHeader := httpReq.Header.Get("Authorization")
		if strings.HasPrefix(authHeader, "Bearer ") {
			rawToken = strings.TrimPrefix(authHeader, "Bearer ")
		}
	}

	// 将 JWT 加入黑名单
	if rawToken != "" {
		if err := s.userCase.AddJwtToBlacklist(ctx, rawToken); err != nil {
			s.log.Errorf("Failed to add JWT to blacklist: %v", err)
			// 即使黑名单添加失败也返回成功，不影响用户登出体验
		}
	}

	return &pb.LogoutReply{}, nil
}

// Auth 用户权限信息
func (s *SysUserService) Auth(ctx context.Context, req *pb.AuthRequest) (*pb.AuthReply, error) {
	if err := req.Validate(); err != nil {
		return nil, err
	}

	user, err := s.userCase.FindSysUserByUsername(ctx, req.Username)
	if err != nil {
		return nil, err
	}
	role, err := s.roleCase.FindRole(ctx, user.RoleID)
	if err != nil {
		return nil, err
	}

	permits, err := s.roleMenuCase.FindPermission(ctx, role.ID)
	if err != nil {
		return nil, err
	}

	var menus []*pb.MenuTree
	// 被禁用了，菜单显示空
	if role.Status == constant.StatusMenusForbidden {
		menus = make([]*pb.MenuTree, 0)
	} else {
		menus, err = s.roleMenuCase.SelectMenuRole(ctx, role.RoleName)
		if err != nil {
			return nil, err
		}
	}

	pbUser := &pb.AuthReply_User{
		UserId:    user.ID,
		NickName:  user.NickName,
		Phone:     user.Phone,
		RoleId:    user.RoleID,
		Avatar:    user.Avatar,
		Sex:       user.Sex,
		Email:     user.Email,
		DeptId:    user.DeptID,
		PostId:    user.PostID,
		RoleIds:   user.RoleIds,
		PostIds:   user.PostIds,
		CreateBy:  user.CreateBy,
		UpdateBy:  user.UpdateBy,
		Remark:    user.Remark,
		Status:    user.Status,
		CreatedAt: util.NewTimestamp(user.CreatedAt),
		UpdatedAt: util.NewTimestamp(user.UpdatedAt),
		Username:  user.Username,
		RoleName:  role.RoleName,
	}

	pbRole := &pb.AuthReply_Role{
		RoleId:    role.ID,
		RoleName:  role.RoleName,
		Status:    role.Status,
		RoleKey:   role.RoleKey,
		RoleSort:  role.RoleSort,
		DataScope: role.DataScope,
		CreateBy:  role.CreateBy,
		UpdateBy:  role.UpdateBy,
		Remark:    role.Remark,
		ApiIds:    nil,
		MenuIds:   nil,
		DeptIds:   nil,
		CreatedAt: util.NewTimestamp(user.CreatedAt),
		UpdatedAt: util.NewTimestamp(user.UpdatedAt),
	}

	return &pb.AuthReply{
		User:        pbUser,
		Role:        pbRole,
		Permissions: permits,
		Menus:       Build(menus),
	}, nil
}

func (s *SysUserService) ChangeStatus(ctx context.Context, req *pb.ChangeStatusRequest) (*pb.ChangeStatusReply, error) {
	if err := req.Validate(); err != nil {
		return nil, err
	}
	err := s.userCase.ChangeStatus(ctx, req.UserId, req.Status)
	return &pb.ChangeStatusReply{}, err
}

func (s *SysUserService) UpdateAvatar(ctx context.Context) error {
	return s.userCase.UpdateAvatar(ctx)
}

func (s *SysUserService) UpdatePassword(ctx context.Context, req *pb.UpdatePasswordRequest) (*pb.UpdatePasswordReply, error) {
	if err := req.Validate(); err != nil {
		return nil, err
	}
	claims := authz.MustFromContext(ctx)
	err := s.userCase.UpdatePassword(ctx, claims.UserID, req.NewPassword, req.OldPassword)
	return &pb.UpdatePasswordReply{}, err
}

// GetPostInit 获取初始化角色岗位信息
func (s *SysUserService) GetPostInit(ctx context.Context, req *pb.FindPostInitRequest) (*pb.FindPostInitReply, error) {
	// 获取所有角色
	roleList, err := s.roleCase.FindRoleAll(ctx)
	if err != nil {
		return nil, err
	}
	// 获取所有岗位
	postList, err := s.postCase.FindPostAll(ctx)
	if err != nil {
		return nil, err
	}

	replyRoles := make([]*pb.RoleData, len(roleList))
	for i, d := range roleList {
		replyRoles[i] = &pb.RoleData{
			RoleId:     d.ID,
			RoleName:   d.RoleName,
			Status:     d.Status,
			RoleKey:    d.RoleKey,
			RoleSort:   d.RoleSort,
			DataScope:  int64(d.DataScope),
			CreateBy:   d.CreateBy,
			UpdateBy:   d.UpdateBy,
			Remark:     d.Remark,
			CreateTime: util.NewTimestamp(d.CreatedAt),
			UpdateTime: util.NewTimestamp(d.UpdatedAt),
		}
	}

	replyPosts := make([]*pb.PostData, len(postList))
	for i, d := range postList {
		replyPosts[i] = &pb.PostData{
			PostId:     d.ID,
			PostName:   d.PostName,
			PostCode:   d.PostCode,
			Sort:       d.Sort,
			Status:     d.Status,
			Remark:     d.Remark,
			CreateBy:   d.CreateBy,
			UpdateBy:   d.UpdateBy,
			CreateTime: util.NewTimestamp(d.CreatedAt),
			UpdateTime: util.NewTimestamp(d.UpdatedAt),
		}
	}

	return &pb.FindPostInitReply{
		Roles: replyRoles,
		Posts: replyPosts,
	}, nil
}

// GetUserRolePost 获取用户角色岗位信息
func (s *SysUserService) GetUserRolePost(ctx context.Context, req *pb.FindUserRolePostRequest) (*pb.FindUserRolePostReply, error) {
	claims := authz.MustFromContext(ctx)
	user, err := s.userCase.FindSysUserById(ctx, claims.UserID)
	if err != nil {
		return nil, err
	}
	roleIds := util.Split2Int64Slice(user.RoleIds)
	postIds := util.Split2Int64Slice(user.PostIds)

	roleList, err := s.roleCase.FindRoleByIDList(ctx, roleIds)
	if err != nil {
		return nil, err
	}
	postList, err := s.postCase.FindPostByIDList(ctx, postIds)
	if err != nil {
		return nil, err
	}

	replyRoles := make([]*pb.RoleData, len(roleList))
	for i, d := range roleList {
		replyRoles[i] = &pb.RoleData{
			RoleId:     d.ID,
			RoleName:   d.RoleName,
			Status:     d.Status,
			RoleKey:    d.RoleKey,
			RoleSort:   d.RoleSort,
			DataScope:  int64(d.DataScope),
			CreateBy:   d.CreateBy,
			UpdateBy:   d.UpdateBy,
			Remark:     d.Remark,
			CreateTime: util.NewTimestamp(d.CreatedAt),
			UpdateTime: util.NewTimestamp(d.UpdatedAt),
		}
	}

	replyPosts := make([]*pb.PostData, len(postList))
	for i, d := range postList {
		replyPosts[i] = &pb.PostData{
			PostId:     d.ID,
			PostName:   d.PostName,
			PostCode:   d.PostCode,
			Sort:       d.Sort,
			Status:     d.Status,
			Remark:     d.Remark,
			CreateBy:   d.CreateBy,
			UpdateBy:   d.UpdateBy,
			CreateTime: util.NewTimestamp(d.CreatedAt),
			UpdateTime: util.NewTimestamp(d.UpdatedAt),
		}
	}

	return &pb.FindUserRolePostReply{
		Roles: replyRoles,
		Posts: replyPosts,
	}, err
}

func (s *SysUserService) GetUserGoogleSecret(ctx context.Context, req *pb.FindUserGoogleSecretRequest) (*pb.FindUserGoogleSecretReply, error) {
	gAuth := util.NewGoogleAuth()
	secret := gAuth.GetSecret()
	qrcode := gAuth.GetQrcode(secret)
	var rep = &pb.FindUserGoogleSecretReply{}
	rep.Secret = secret
	rep.Qrcode = qrcode
	return rep, nil
}

func (s *SysUserService) UploadFile(ctx context.Context) (string, error) {
	return s.userCase.UploadFile(ctx)
}
