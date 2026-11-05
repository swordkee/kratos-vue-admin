<template>
  <div class="app-container">
    <el-card shadow="always">
      <!--查询-->
      <el-form :model="state.queryParams" ref="queryForm" :inline="true">
        <el-form-item label="角色名称" prop="roleName">
          <el-input placeholder="角色名称模糊查询" clearable @keyup.enter="handleQuery" style="width: 240px"
            v-model="state.queryParams.roleName" />
        </el-form-item>
        <el-form-item label="权限字符" prop="roleKey">
          <el-input placeholder="权限字符模糊查询" clearable @keyup.enter="handleQuery" style="width: 240px"
            v-model="state.queryParams.roleKey" />
        </el-form-item>
        <el-form-item label="状态" prop="status">
          <el-select v-model="state.queryParams.status" placeholder="角色状态" clearable style="width: 240px">
            <el-option v-for="dict in state.statusOptions" :key="dict.dictValue" :label="dict.dictLabel"
              :value="dict.dictValue" />
          </el-select>
        </el-form-item>
        <el-form-item>
          <el-button type="primary" plain @click="handleQuery">
            <SvgIcon name="elementSearch" />
            搜索
          </el-button>
          <el-button @click="resetQuery">
            <SvgIcon name="elementRefresh" />
            重置
          </el-button>
        </el-form-item>
      </el-form>
    </el-card>
    <el-card class="box-card">
      <template #header>
        <div class="card-header">
          <span class="card-header-text">角色列表</span>
          <div>
            <el-button type="primary" plain v-auth="'system:role:add'" :disabled="state.permissionLoading" @click="openRoleForm()">
              <SvgIcon name="elementPlus" />
              新增
            </el-button>
            <el-button type="danger" plain v-auth="'system:role:delete'" :disabled="state.multiple" @click="handleDelete">
              <SvgIcon name="elementDelete" />
              删除
            </el-button>
          </div>
        </div>
      </template>
      <!--数据表格-->
      <el-table v-loading="state.loading" :data="state.roleList" @selection-change="handleSelectionChange">
        <el-table-column type="selection" width="55" align="center" />
        <el-table-column label="编号" prop="roleId" />
        <el-table-column label="角色名称" prop="roleName" :show-overflow-tooltip="true" />
        <el-table-column label="权限字符" prop="roleKey" :show-overflow-tooltip="true" />
        <el-table-column label="显示顺序" prop="roleSort" />
        <el-table-column label="状态" align="center">
          <template #default="scope">
            <el-switch v-model="scope.row.status" :active-value="1" :inactive-value="2"
              @click="handleStatusChange(scope.row)"></el-switch>
          </template>
        </el-table-column>
        <el-table-column label="创建时间" align="center" prop="createTime">
          <template #default="scope" v-auth="'system:role:edit'">
            <span>{{ dateStrFormat(scope.row?.createTime) }}</span>
          </template>
        </el-table-column>
        <el-table-column label="操作" align="center">
          <template #default="scope">
            <el-popover placement="left">
              <template #reference>
                <el-button type="primary" circle>
                  <SvgIcon name="elementStar" />
                </el-button>
              </template>
              <div>
                <el-button text type="primary" v-auth="'system:role:edit'" :disabled="state.permissionLoading" @click="openRoleForm(scope.row)">
                  <SvgIcon name="elementEdit" />
                  修改
                </el-button>
              </div>
              <div>
                <el-button text type="primary" @click="handleDataScope(scope.row)">
                  <SvgIcon name="elementCircleCheck" />
                  数据权限
                </el-button>
              </div>
              <div>
                <el-button text type="primary" v-auth="'system:role:delete'" @click="handleDelete(scope.row)">
                  <SvgIcon name="elementDelete" />
                  删除
                </el-button>
              </div>
            </el-popover>
          </template>
        </el-table-column>
      </el-table>
      <!-- 分页设置-->
      <div v-show="state.total > 0">
        <el-divider></el-divider>
        <el-pagination background :total="state.total" :page-sizes="[10, 20, 30]" :current-page.sync="state.queryParams.pageNum"
          :page-size="state.queryParams.pageSize" layout="total, sizes, prev, pager, next, jumper"
          @size-change="handleSizeChange" @current-change="handleCurrentChange" />
      </div>
    </el-card>
    <!-- 添加或修改角色配置对话框 -->
    <el-dialog :title="state.title" v-model="state.open" width="min(769px, 95vw)" center destroy-on-close @opened="restoreRoleChecks">
      <div v-drag="['.app-container .el-dialog', '.app-container .el-dialog__header']"></div>
      <el-form ref="roleFormRef" :model="state.roleForm" :rules="state.rules" label-width="120px" label-position="right">
        <el-form-item label="角色名称" prop="roleName">
          <el-input v-model="state.roleForm.roleName" placeholder="请输入角色名称" />
        </el-form-item>
        <el-form-item label="权限字符" prop="roleKey">
          <el-input v-model="state.roleForm.roleKey" placeholder="请输入权限字符" />
        </el-form-item>
        <el-form-item label="角色顺序" prop="roleSort">
          <el-input-number v-model="state.roleForm.roleSort" controls-position="right" :min="0" />
        </el-form-item>
        <el-form-item label="状态">
          <el-radio-group v-model="state.roleForm.status">
            <el-radio v-for="dict in state.statusOptions" :key="dict.dictValue" :label="Number(dict.dictValue)">{{ dict.dictLabel
            }}
            </el-radio>
          </el-radio-group>
        </el-form-item>

        <el-form-item label="权限设置">
          <el-tabs v-model="state.activeName">
            <el-tab-pane label="菜单权限" name="first">
              <el-checkbox v-model="state.menuExpand" @change="handleCheckedTreeExpand($event, 'menu')">展开/折叠
              </el-checkbox>
              <el-checkbox v-model="state.menuNodeAll" @change="handleCheckedTreeNodeAll($event, 'menu')">全选/全不选
              </el-checkbox>
              <el-tree class="tree-border" :data="state.menuOptions" show-checkbox ref="menuRef" node-key="menuId" @check="state.menuChanged = true"
                empty-text="暂无可分配菜单" :props="{
                  label: 'menuName',
                  children: 'children',
                }"></el-tree>
            </el-tab-pane>
            <el-tab-pane label="API权限" name="second">
              <el-checkbox v-model="state.apiExpand" @change="handleCheckedTreeExpand($event, 'api')">展开/折叠
              </el-checkbox>
              <el-checkbox v-model="state.apiNodeAll" @change="handleCheckedTreeNodeAll($event, 'api')">全选/全不选
              </el-checkbox>
              <el-tree class="tree-border" :data="state.apiOptions" show-checkbox ref="apiRef" node-key="onlyId"
                empty-text="暂无已登记接口" :props="{
                  children: 'children',
                  label: 'description'
                }" :default-checked-keys="state.apiCheckedKeys"></el-tree>
            </el-tab-pane>
          </el-tabs>
        </el-form-item>
        <el-form-item label="备注">
          <el-input v-model="state.roleForm.remark" type="textarea" placeholder="请输入内容"></el-input>
        </el-form-item>
      </el-form>
      <template #footer>
        <span class="dialog-footer">
          <el-button type="primary" :loading="state.bunLoading" :disabled="!state.permissionsReady" @click="submitForm">确 定</el-button>
          <el-button @click="cancel">取 消</el-button>
        </span>
      </template>
    </el-dialog>

    <!-- 分配角色数据权限对话框 -->
    <el-dialog :title="state.title" v-model="state.openDataScope" width="769px">
      <div v-drag="['.app-container .el-dialog', '.app-container .el-dialog__header']"></div>
      <el-form :model="state.roleForm" label-width="80px">
        <el-form-item label="角色名称">
          <el-input v-model="state.roleForm.roleName" :disabled="true" />
        </el-form-item>
        <el-form-item label="权限字符">
          <el-input v-model="state.roleForm.roleKey" :disabled="true" />
        </el-form-item>
        <el-form-item label="权限范围">
          <el-select v-model="state.roleForm.dataScope" @change="dataScopeSelectChange">
            <el-option v-for="item in state.dataScopeOptions" :key="item.value" :label="item.label"
              :value="item.value"></el-option>
          </el-select>
        </el-form-item>
        <el-form-item label="数据权限" v-show="state.roleForm.dataScope == 2">
          <el-checkbox v-model="state.deptExpand" @change="handleCheckedTreeExpand($event, 'dept')">展开/折叠
          </el-checkbox>
          <el-checkbox v-model="state.deptNodeAll" @change="handleCheckedTreeNodeAll($event, 'dept')">全选/全不选
          </el-checkbox>
          <el-tree class="tree-border" :data="state.deptOptions" show-checkbox default-expand-all ref="deptRef"
            node-key="deptId" empty-text="加载中，请稍后" :props="{
              label: 'deptName',
              children: 'children',
            }"></el-tree>
        </el-form-item>
      </el-form>
      <template #footer>
        <span class="dialog-footer">
          <el-button type="primary" @click="submitDataScope">确 定</el-button>
          <el-button @click="cancelDataScope">取 消</el-button>
        </span>
      </template>
    </el-dialog>
  </div>
</template>

<script lang="ts" setup>
import {
  getCurrentInstance,
  onMounted,
  reactive,
  ref,
  toRefs,
  unref,
  nextTick,
} from "vue";
import { ElMessage, ElMessageBox } from "element-plus";
import {
  listRole,
  getRole,
  delRole,
  dataScope,
  addRole,
  updateRole,
  exportRole,
  changeRoleStatus,
} from "@/api/system/role";
import { roleMenuTreeselect } from "@/api/system/menu";
import {
  listApiAll, getPolicyPathByRoleId
} from "@/api/system/api";
import {
  roleDeptTreeselect,
} from "@/api/system/dept";
import { handleFileError } from "@/utils/export";
import DictList from "@/views/system/dict/component/dictList.vue";

const { proxy } = getCurrentInstance() as any;
const roleFormRef = ref<HTMLElement | null>(null);
const menuRef = ref<HTMLElement | null>(null);
const apiRef = ref<HTMLElement | null>(null);
const deptRef = ref<HTMLElement | null>(null);
const state: any = reactive({
  activeName: 'first',
  // 遮罩层
  loading: true,
  bunLoading: false,
  permissionLoading: false,
  permissionsReady: false,
  menuChanged: false,
  // 选中数组
  ids: [],
  // 总条数
  total: 0,
  // 弹出层标题
  title: "",
  // 是否显示弹出层
  open: false,
  // 角色表格数据
  roleList: [],
  // 显示搜索条件
  showSearch: true,
  // 非单个禁用
  single: true,
  // 非多个禁用
  multiple: true,
  // 状态数据字典
  statusOptions: [],
  // 菜单列表
  menuOptions: [],
  // 菜单列表
  apiOptions: [],
  //角色具有的菜单列表
  menuCheckedKeys: [],
  //角色具有的api列表
  apiCheckedKeys: [],
  // 部门列表
  deptOptions: [],
  // 是否显示弹出层（数据权限）
  openDataScope: false,
  menuExpand: false,
  menuNodeAll: false,
  apiExpand: false,
  apiNodeAll: false,
  deptExpand: true,
  deptNodeAll: false,
  // 查询参数
  queryParams: {
    pageNum: 1,
    pageSize: 10,
    roleName: undefined,
    roleKey: undefined,
    status: undefined,
  },
  // 数据范围选项
  dataScopeOptions: [
    {
      value: "1",
      label: "全部数据权限",
    },
    {
      value: "2",
      label: "自定数据权限",
    },
    {
      value: "3",
      label: "本部门数据权限",
    },
    {
      value: "4",
      label: "本部门及以下数据权限",
    },
    {
      value: "5",
      label: "仅本人数据权限",
    },
  ],
  // 表单参数
  roleForm: {},
  // 表单校验
  rules: {
    roleName: [
      { required: true, message: "角色名称不能为空", trigger: "blur" },
    ],
    roleKey: [
      { required: true, message: "权限字符不能为空", trigger: "blur" },
    ],
    roleSort: [
      { required: true, message: "角色顺序不能为空", trigger: "blur" },
    ],
  },
});
// 表单重置
const reset = () => {
  state.menuExpand = state.menuNodeAll = false;
  state.apiExpand = state.apiNodeAll = false;
  state.deptExpand = true;
  state.deptNodeAll = false;
  state.roleForm = { roleSort: 0, status: 1 };
};
/** 重置按钮操作 */
const resetQuery = () => {
  state.queryParams.pageNum = 1;
  state.queryParams.pageSize = 10;
  state.queryParams.roleName = "";
  state.queryParams.roleKey = "";
  state.queryParams.status = undefined;
  handleQuery();
};
/** 查询角色列表 */
const getList = () => {
  state.loading = true;
  listRole(state.queryParams).then(
    (response) => {
      state.roleList = response.data;
      state.total = response.total;
      state.loading = false;
    }
  );
};
//分页页面大小发生变化
const handleSizeChange = (val: any) => {
  state.queryParams.pageSize = val;
  handleQuery();
};
//当前页码发生变化
const handleCurrentChange = (val: any) => {
  state.queryParams.pageNum = val;
  handleQuery();
};
/** 提交按钮（数据权限） */
const submitDataScope = () => {
  if (state.roleForm.roleId != undefined) {
    state.roleForm.deptIds = getDeptAllCheckedKeys();
    dataScope(state.roleForm).then((response) => {
      ElMessage.success("修改成功");
      state.openDataScope = false;
      getList();
    });
  }
};
// 所有部门节点数据
const getDeptAllCheckedKeys = () => {
  const formWrap = unref(deptRef) as any;
  // 目前被选中的部门节点
  let checkedKeys = formWrap.getCheckedKeys();
  // 半选中的部门节点
  let halfCheckedKeys = formWrap.getHalfCheckedKeys();
  checkedKeys.unshift.apply(checkedKeys, halfCheckedKeys);
  return checkedKeys;
};
/** 搜索按钮操作 */
const handleQuery = () => {
  // state.queryParams.pageNum = 1;
  getList();
};
type RoleApi = { path: string; method: string; apiGroup?: string; description?: string };
const apiKey = (api: RoleApi) => `p:${api.path}m:${api.method}`;

function buildApiTree(catalog: RoleApi[], granted: RoleApi[]) {
  const groups = new Map<string, any[]>();
  const seen = new Set<string>();
  // Keep existing grants selectable when the API was never registered or its catalog entry was removed.
  for (const api of [...catalog, ...granted]) {
    const onlyId = apiKey(api);
    if (seen.has(onlyId)) continue;
    seen.add(onlyId);
    const group = api.apiGroup || '其他接口';
    if (!groups.has(group)) groups.set(group, []);
    groups.get(group)!.push({ ...api, onlyId, description: api.description || `${api.method} ${api.path}` });
  }
  return Array.from(groups, ([group, children]) => ({ onlyId: group, description: group + '组', children }));
}

// 多选框选中数据
const handleSelectionChange = (selection: any) => {
  state.ids = selection.map((item: any) => item.roleId);
  state.single = selection.length != 1;
  state.multiple = !selection.length;
};
// 角色状态修改
const handleStatusChange = (row: any) => {
  // console.log(row.status)
  let text = row.status == 1 ? "启用" : "停用";
  ElMessageBox({
    closeOnClickModal: false,
    closeOnPressEscape: false,
    title: "警告",
    message: '确认要"' + text + '""' + row.roleName + '"角色吗?',
    showCancelButton: true,
    confirmButtonText: "确定",
    cancelButtonText: "取消",
  })
    .then(function () {
      return changeRoleStatus(row.roleId, row.status);
    })
    .then(() => {
      ElMessage.success(text + "成功");
    })
    .catch((action: Action) => {
      if (action === 'cancel') {
        // 状态域为 1=正常 / 2=停用（DDL 注释、字典 sys_normal_disable、Go 常量
        // constant.StatusUserForbidden=2 三者一致），取消时回滚
        row.status = row.status == 1 ? 2 : 1
      }
    });
};
/** 分配数据权限操作 */
const handleDataScope = (row: any) => {
  reset();
  const roleDeptTreeselect = getRoleDeptTreeselect(row.roleId);
  getRole(row.roleId).then((response) => {
    state.roleForm = response?.role;
    state.openDataScope = true;
    nextTick(() => {
      roleDeptTreeselect.then((res) => {
        const formWrap = unref(deptRef) as any;
        formWrap.setCheckedKeys(res.checkedKeys);
      });
    });
    state.title = "分配数据权限";
  });
};
/** Wait for the catalog and the role's current grants before allowing a full replacement. */
async function openRoleForm(row?: any) {
  if (state.permissionLoading) return;
  reset();
  state.activeName = 'first';
  state.permissionsReady = false;
  state.permissionLoading = true;
  state.menuChanged = false;
  try {
    const roleAndPolicies = row
      ? getRole(row.roleId).then(async ({ role }: any) => ({
          role, grants: (await getPolicyPathByRoleId({ roleKey: role.roleKey }) as any).apis,
        }))
      : { role: state.roleForm, grants: [] };
    const [{ role, grants }, menu, catalog] = await Promise.all([
      roleAndPolicies, roleMenuTreeselect(row?.roleId ?? 0), listApiAll({}),
    ]) as any;
    if (![catalog.data, grants, menu.menus, menu.checkedKeys].every(Array.isArray)) {
      throw new Error('权限数据不完整');
    }
    state.roleForm = role;
    state.menuOptions = menu.menus;
    state.menuCheckedKeys = menu.checkedKeys;
    state.apiOptions = buildApiTree(catalog.data, grants);
    state.apiCheckedKeys = grants.map(apiKey);
    state.title = row ? '修改角色' : '添加角色';
    state.open = true;
  } catch {
    ElMessage.error('角色权限未加载完整，请重试');
  } finally {
    state.permissionLoading = false;
  }
}

function restoreRoleChecks() {
  const tree = unref(menuRef) as any;
  if (!tree || !unref(apiRef)) return;
  // The stored set includes parent directories. Checking a parent recursively would grant its other children.
  const nodes = state.menuCheckedKeys.map((id: number) => tree.getNode(id)).filter(Boolean);
  nodes.sort((a: any, b: any) => a.level - b.level);
  for (const node of nodes) tree.setChecked(node.key, true, false);
  state.permissionsReady = true;
}
/** 删除按钮操作 */
const handleDelete = (row: any) => {
  const roleIds = row.roleId || state.ids;
  ElMessageBox({
    message: '是否确认删除角色编号为"' + roleIds + '"的数据项?',
    title: "警告",
    showCancelButton: true,
    confirmButtonText: "确定",
    cancelButtonText: "取消",
  })
    .then(function () {
      return delRole(roleIds);
    })
    .then(() => {
      getList();
      ElMessage.success("删除成功");
    });
};
/** 导出按钮操作 */
const handleExport = () => {
  const queryParams = state.queryParams;
  let data: any = new Date().getTime() / 1000
  let time = parseInt(data) + '';
  queryParams.filename = "角色表_" + time + ".xlsx"
  ElMessageBox({
    message: "是否确认导出所有角色数据项?",
    title: "警告",
    showCancelButton: true,
    confirmButtonText: "确定",
    cancelButtonText: "取消",
  })
    .then(function () {
      return exportRole(queryParams);
    })
    .then((response: any) => {
      handleFileError(response, queryParams.filename)
    });
};
/** 根据角色ID查询部门树结构 */
const getRoleDeptTreeselect = (roleId: number) => {
  return roleDeptTreeselect(roleId).then((response) => {
    state.deptOptions = response.depts;
    return response;
  });
};
/** 提交按钮 */
const submitForm = async () => {
  const form = unref(roleFormRef) as any;
  if (!form || !state.permissionsReady || state.bunLoading) return;
  state.bunLoading = true;
  if (!await form.validate().catch(() => false)) {
    state.bunLoading = false;
    return;
  }
  const role = state.roleForm;
  // The HTTP client adds snake_case aliases on read. Send only canonical fields from the edited form.
  const data = {
    roleId: role.roleId, roleKey: role.roleKey, roleName: role.roleName, sort: role.roleSort,
    status: Number(role.status), dataScope: Number(role.dataScope || 0), defaultRouter: role.defaultRouter,
    remark: role.remark, menuIds: state.menuChanged ? getMenuAllCheckedKeys() : state.menuCheckedKeys,
    apiIds: getApiAllCheckedKeys(),
  };
  try {
    if (role.roleId) await updateRole(data);
    else await addRole(data);
    ElMessage.success(role.roleId ? "修改成功" : "新增成功");
    state.open = false;
    getList();
  } finally {
    state.bunLoading = false;
  }
};
// 取消按钮
const cancel = () => {
  state.open = false;
  reset();
};
// 所有菜单节点数据
const getMenuAllCheckedKeys = () => {
  const formWrap = unref(menuRef) as any;
  // 目前被选中的菜单节点
  let checkedKeys = formWrap.getCheckedKeys();
  // 半选中的菜单节点
  let halfCheckedKeys = formWrap.getHalfCheckedKeys();
  const absent = state.menuCheckedKeys.filter((id: number) => !formWrap.getNode(id));
  return [...new Set([...checkedKeys, ...halfCheckedKeys, ...absent])];
};
// 所有菜单节点数据
const getApiAllCheckedKeys = () => {
  const formWrap = unref(apiRef) as any;
  return formWrap.getCheckedNodes(true).map(({ path, method }: RoleApi) => ({ path, method }));
};

// 取消按钮（数据权限）
const cancelDataScope = () => {
  state.openDataScope = false;
  reset();
};
/** 选择角色权限范围触发 */
const dataScopeSelectChange = (value: any) => {
  const formWrap = unref(deptRef) as any;
  if (value !== "2") {
    formWrap.setCheckedKeys([]);
  }
};
// 树权限（展开/折叠）
const handleCheckedTreeExpand = (value: any, type: any) => {
  if (type == "menu") {
    let treeList = state.menuOptions;
    for (let i = 0; i < treeList.length; i++) {
      const formWrap = unref(menuRef) as any;
      console.log("treeList", treeList[i])
      formWrap.store.nodesMap[treeList[i].menuId].expanded = value;
    }
  } else if (type == "api") {
    let treeList = state.apiOptions;
    for (let i = 0; i < treeList.length; i++) {
      const formWrap = unref(apiRef) as any;
      console.log("treeList", treeList[i])
      console.log("formWrap", formWrap)
      formWrap.store.nodesMap[treeList[i].onlyId].expanded = value;
    }
  } else if (type == "dept") {
    let treeList = state.deptOptions;
    for (let i = 0; i < treeList.length; i++) {
      const formWrap = unref(deptRef) as any;
      formWrap.store.nodesMap[treeList[i].deptId].expanded = value;
    }
  }
};
// 树权限（全选/全不选）
const handleCheckedTreeNodeAll = (value: any, type: any) => {
  if (type == "menu") {
    state.menuChanged = true;
    const formWrap = unref(menuRef) as any;
    formWrap.setCheckedNodes(value ? state.menuOptions : []);
  } else if (type == "api") {
    const formWrap = unref(apiRef) as any;
    formWrap.setCheckedNodes(value ? state.apiOptions : []);
  } else if (type == "dept") {
    const formWrap = unref(deptRef) as any;
    formWrap.setCheckedNodes(value ? state.deptOptions : []);
  }
};
// 页面加载时
onMounted(() => {
  getList();
  // 查询菜单状态数据字典
  proxy.getDicts("sys_normal_disable").then((response: any) => {
    console.log(response.list, "response.data");
    state.statusOptions = response?.list;
  });
});

</script>

<style scoped></style>
