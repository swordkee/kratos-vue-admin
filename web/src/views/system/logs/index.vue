<template>
  <div class="system-operation-record-container app-container">
    <el-card shadow="always">
      <!-- 查询 -->
      <el-form :model="state.queryParams" ref="queryForm" :inline="true" label-width="78px">
        <el-form-item label="请求IP" prop="ip">
          <el-input placeholder="请输入请求IP模糊查询" clearable @keyup.enter="handleQuery" style="width: 200px"
            v-model="state.queryParams.ip" />
        </el-form-item>
        <el-form-item label="请求方法" prop="method">
          <el-select v-model="state.queryParams.method" placeholder="请选择请求方法" clearable style="width: 150px">
            <el-option v-for="method in state.methodOptions" :key="method" :label="method" :value="method" />
          </el-select>
        </el-form-item>
        <el-form-item label="请求路径" prop="path">
          <el-input placeholder="请输入请求路径模糊查询" clearable @keyup.enter="handleQuery" style="width: 240px"
            v-model="state.queryParams.path" />
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

    <!-- 操作按钮 -->
    <el-card class="box-card">
      <template #header>
        <div class="card-header">
          <span class="card-header-text">操作日志</span>
          <div>
            <el-button type="danger" plain :disabled="state.multiple" @click="onTabelRowDel">
              <SvgIcon name="elementDelete" />
              删除
            </el-button>
          </div>
        </div>
      </template>

      <!-- 数据表格 -->
      <el-table v-loading="state.loading" :data="state.tableData" stripe @selection-change="handleSelectionChange"
        style="width: 100%">
        <el-table-column type="selection" width="45" align="center" />
        <el-table-column label="编号" align="center" prop="id" width="80" />
        <el-table-column label="请求IP" align="center" prop="ip" width="140" :show-overflow-tooltip="true" />
        <el-table-column label="请求方法" align="center" prop="method" width="100">
          <template #default="scope">
            <el-tag :type="getMethodType(scope.row.method)" size="small">
              {{ scope.row.method }}
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column label="请求路径" align="center" prop="path" :show-overflow-tooltip="true" />
        <el-table-column label="请求" align="center" width="80">
          <template #default="scope">
            <div>
              <el-popover
                v-if="scope.row.body"
                placement="left-start"
                :width="444"
                trigger="click"
              >
                <template #reference>
                  <el-icon style="cursor: pointer; color: #409eff"><View /></el-icon>
                </template>
                <div class="popover-box">
                  <h4>请求 Body</h4>
                  <pre>{{ fmtBody(scope.row.body) }}</pre>
                </div>
              </el-popover>
              <span v-else style="color: #909399">无</span>
            </div>
          </template>
        </el-table-column>
        <el-table-column label="响应" align="center" width="80">
          <template #default="scope">
            <div>
              <el-popover
                v-if="scope.row.resp"
                placement="left-start"
                :width="444"
                trigger="click"
              >
                <template #reference>
                  <el-icon style="cursor: pointer; color: #67c23a"><View /></el-icon>
                </template>
                <div class="popover-box">
                  <h4>响应 Body</h4>
                  <pre>{{ fmtBody(scope.row.resp) }}</pre>
                </div>
              </el-popover>
              <span v-else style="color: #909399">无</span>
            </div>
          </template>
        </el-table-column>
        <el-table-column label="状态" align="center" prop="status" width="110">
          <template #default="scope">
            <el-tag :type="statusTagType(scope.row.status)" size="small">
              {{ statusText(scope.row.status) }}
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column label="延迟(ms)" align="center" prop="latency" width="100" />
        <el-table-column label="操作时间" align="center" width="180">
          <template #default="scope">
            {{ scope.row.created_at }}
          </template>
        </el-table-column>
        <el-table-column label="操作" align="center" width="100">
          <template #default="scope">
            <el-button text type="primary" @click="handleDetail(scope.row)">
              <SvgIcon name="elementView" />
              详情
            </el-button>
          </template>
        </el-table-column>
      </el-table>

      <div v-show="state.total > 0">
        <el-divider></el-divider>
        <el-pagination background :total="state.total" :current-page="state.queryParams.pageNum"
          :page-sizes="[10, 20, 30, 50]" :page-size="state.queryParams.pageSize"
          layout="total, sizes, prev, pager, next, jumper" @size-change="handleSizeChange"
          @current-change="handleCurrentChange" />
      </div>
    </el-card>

    <!-- 详情对话框 -->
    <el-dialog title="操作日志详情" v-model="state.detailVisible" width="70%" append-to-body>
      <el-descriptions :column="2" border>
        <el-descriptions-item label="编号">{{ state.currentRow.id }}</el-descriptions-item>
        <el-descriptions-item label="请求IP">{{ state.currentRow.ip }}</el-descriptions-item>
        <el-descriptions-item label="请求方法">{{ state.currentRow.method }}</el-descriptions-item>
        <el-descriptions-item label="请求路径">{{ state.currentRow.path }}</el-descriptions-item>
        <el-descriptions-item label="状态">
          <el-tag :type="statusTagType(state.currentRow.status)">
            {{ statusText(state.currentRow.status) }}
          </el-tag>
        </el-descriptions-item>
        <el-descriptions-item label="延迟">{{ state.currentRow.latency }}ms</el-descriptions-item>
        <el-descriptions-item label="操作时间">{{ state.currentRow.created_at }}</el-descriptions-item>
        <el-descriptions-item label="User-Agent" :span="2">{{ state.currentRow.agent }}</el-descriptions-item>
        <el-descriptions-item label="错误信息" :span="2">
          <span v-if="state.currentRow.error_message" style="color: #f56c6c">{{ state.currentRow.error_message }}</span>
          <span v-else>-</span>
        </el-descriptions-item>
        <el-descriptions-item label="请求Body" :span="2">
          <el-input type="textarea" :rows="4" readonly :value="state.currentRow.body || '-'" />
        </el-descriptions-item>
        <el-descriptions-item label="响应Body" :span="2">
          <el-input type="textarea" :rows="4" readonly :value="state.currentRow.resp || '-'" />
        </el-descriptions-item>
      </el-descriptions>
    </el-dialog>
  </div>
</template>

<script lang="ts" setup name="SystemLogs">
  import { reactive, onMounted } from "vue";
  import { View } from "@element-plus/icons-vue";
  import { listOperationRecord, delOperationRecordByIds } from "@/api/system/logs";
  import { ElMessageBox, ElMessage } from "element-plus";

  const state = reactive({
    // 遮罩层
    loading: true,
    // 总条数
    total: 0,
    // 列表表格数据
    tableData: [] as any[],
    // 选中数组
    ids: [] as any[],
    // 非多个禁用
    multiple: true,
    // 查询参数
    queryParams: {
      pageNum: 1,
      pageSize: 10,
      ip: undefined as string | undefined,
      method: undefined as string | undefined,
      path: undefined as string | undefined,
    },
    // 请求方法选项
    methodOptions: ['GET', 'POST', 'PUT', 'DELETE', 'PATCH'],
    // 详情弹窗
    detailVisible: false,
    // 当前行数据
    currentRow: {} as any,
  });

  /** 查询操作日志列表 */
  const handleQuery = () => {
    state.loading = true;
    listOperationRecord(state.queryParams).then((response: any) => {
      state.tableData = response.list || response.data || [];
      state.total = Number(response.total || 0);
      state.loading = false;
    }).catch(() => {
      state.loading = false;
    });
  };

  /** 重置按钮操作 */
  const resetQuery = () => {
    state.queryParams.ip = undefined;
    state.queryParams.method = undefined;
    state.queryParams.path = undefined;
    handleQuery();
  };

  /** 删除按钮操作 */
  const onTabelRowDel = (row: any) => {
    const ids = row.id || state.ids;
    ElMessageBox({
      message: '是否确认删除编号为"' + ids + '"的数据项?',
      title: "警告",
      showCancelButton: true,
      confirmButtonText: "确定",
      cancelButtonText: "取消",
      type: "warning",
    }).then(function () {
      return delOperationRecordByIds(ids.join(',')).then(() => {
        handleQuery();
        ElMessage.success("删除成功");
      });
    });
  };

  /** 查看详情 */
  const handleDetail = (row: any) => {
    state.currentRow = row;
    state.detailVisible = true;
  };

  /** 分页页面大小发生变化 */
  const handleSizeChange = (val: number) => {
    state.queryParams.pageSize = val;
    handleQuery();
  };
  /** 当前页码发生变化 */
  const handleCurrentChange = (val: number) => {
    state.queryParams.pageNum = val;
    handleQuery();
  };

  /** 多选框选中数据 */
  const handleSelectionChange = (selection: any) => {
    state.ids = selection.map((item: any) => item.id);
    state.multiple = !selection.length;
  };

  /** 根据请求方法返回标签类型 */
  const getMethodType = (method: string) => {
    const methodMap: Record<string, string> = {
      'GET': 'success',
      'POST': 'primary',
      'PUT': 'warning',
      'DELETE': 'danger',
      'PATCH': 'info',
    };
    return methodMap[method?.toUpperCase()] || 'info';
  };

  // 状态语义（go-exAdmin 的 OperationRecord 中间件：成功写 200、失败写错误码/500；
  // 0 为早期未记录状态的存量行）。与 go-payAdmin（其约定 0=成功）不同，按本仓约定渲染。
  const statusText = (status: any) => {
    if (status === 200) return '成功';
    if (!status) return '-';
    return '失败(' + status + ')';
  };
  const statusTagType = (status: any) => {
    if (status === 200) return 'success';
    if (!status) return 'info';
    return 'danger';
  };

  /** 格式化 body 内容 */
  const fmtBody = (body: any) => {
    if (!body) return '-';
    if (typeof body === 'string') {
      try {
        body = JSON.parse(body);
      } catch {
        return body;
      }
    }
    return JSON.stringify(body, null, 2);
  };

  /** 页面加载时调用 */
  onMounted(() => {
    handleQuery();
  });
</script>

<style scoped>
.system-operation-record-container {
  min-height: calc(100vh - 100px);
}

.card-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
}

.card-header-text {
  font-weight: 600;
  font-size: 16px;
}

.box-card {
  margin-top: 15px;
}

.popover-box {
  max-height: 400px;
  overflow: auto;
}

.popover-box h4 {
  margin: 0 0 10px 0;
  font-size: 14px;
  color: #606266;
}

.popover-box pre {
  margin: 0;
  white-space: pre-wrap;
  word-wrap: break-word;
  font-size: 12px;
  line-height: 1.5;
}
</style>
