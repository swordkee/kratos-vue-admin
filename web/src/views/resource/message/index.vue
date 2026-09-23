<template>
  <div class="message-container app-container">
    <el-card shadow="always">
      <template #header>
        <div class="card-header">
          <span class="card-header-text">消息中心（统一站内信）</span>
          <div class="header-actions">
            <el-badge :value="unread" :hidden="unread <= 0" class="unread-badge">
              <el-tag type="danger" effect="plain">未读 {{ unread }}</el-tag>
            </el-badge>
            <el-button type="primary" plain :loading="markAllLoading" @click="handleMarkAllRead">
              全部已读
            </el-button>
            <el-button @click="handleRefresh"><SvgIcon name="elementRefresh" />刷新</el-button>
          </div>
        </div>
      </template>

      <!-- 类型 Tab：全部 / 系统 / 订单 / 结算 / 投诉 -->
      <el-tabs v-model="activeType" @tab-change="handleTabChange">
        <el-tab-pane label="全部" name="all" />
        <el-tab-pane label="系统" name="system" />
        <el-tab-pane label="订单" name="order" />
        <el-tab-pane label="结算" name="settle" />
        <el-tab-pane label="投诉" name="complain" />
      </el-tabs>

      <el-form :model="queryParams" ref="queryForm" :inline="true" label-width="68px">
        <el-form-item label="状态" prop="is_read">
          <el-select v-model="queryParams.is_read" style="width: 160px" @change="handleQuery">
            <el-option label="全部" :value="-1" />
            <el-option label="未读" :value="0" />
            <el-option label="已读" :value="1" />
          </el-select>
        </el-form-item>
        <el-form-item>
          <el-button type="primary" plain @click="handleQuery"><SvgIcon name="elementSearch" />搜索</el-button>
          <el-button @click="handleReset"><SvgIcon name="elementRefresh" />重置</el-button>
        </el-form-item>
      </el-form>

      <!-- 数据表格 -->
      <el-table v-loading="state.loading" :data="state.tableData" border stripe>
        <el-table-column label="类型" align="center" width="90">
          <template #default="{ row }">
            <el-tag :type="typeTag(row.type)" size="small" disable-transitions>{{ typeText(row.type) }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column label="标题" prop="title" min-width="220" :show-overflow-tooltip="true" />
        <el-table-column label="发送人" align="center" width="130">
          <template #default="{ row }">{{ row.sender_name || '系统' }}</template>
        </el-table-column>
        <el-table-column label="状态" align="center" width="90">
          <template #default="{ row }">
            <el-tag :type="row.is_read === 1 ? 'info' : 'danger'" size="small" disable-transitions>
              {{ row.is_read === 1 ? '已读' : '未读' }}
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column label="发送时间" align="center" width="170">
          <template #default="{ row }">{{ formatTime(row.created_at) }}</template>
        </el-table-column>
        <el-table-column label="已读时间" align="center" width="170">
          <template #default="{ row }">{{ formatTime(row.read_at) }}</template>
        </el-table-column>
        <el-table-column label="操作" align="center" width="230" fixed="right">
          <template #default="{ row }">
            <el-button type="primary" link size="small" @click="handleView(row)">查看</el-button>
            <el-button
                v-if="row.is_read === 0"
                type="success"
                link
                size="small"
                @click="handleMarkRead(row)"
            >标记已读</el-button>
            <el-button type="danger" link size="small" @click="handleDelete(row)">删除</el-button>
          </template>
        </el-table-column>
      </el-table>

      <!-- 分页 -->
      <div v-show="state.total > 0">
        <el-divider></el-divider>
        <el-pagination
            background
            :total="state.total"
            :current-page="queryParams.page"
            :page-sizes="[10, 20, 30, 40]"
            :page-size="queryParams.page_size"
            layout="total, sizes, prev, pager, next, jumper"
            @size-change="handleSizeChange"
            @current-change="handleCurrentChange"
        />
      </div>
    </el-card>

    <!-- 详情弹窗（打开未读消息会自动置为已读） -->
    <el-dialog v-model="state.detailVisible" title="消息详情" width="600px" @closed="handleDetailClosed">
      <el-descriptions v-if="state.detail" :column="1" border>
        <el-descriptions-item label="标题">{{ state.detail.title }}</el-descriptions-item>
        <el-descriptions-item label="类型">{{ typeText(state.detail.type) }}</el-descriptions-item>
        <el-descriptions-item label="发送人">{{ state.detail.sender_name || '系统' }}</el-descriptions-item>
        <el-descriptions-item label="状态">
          <el-tag :type="state.detail.is_read === 1 ? 'info' : 'danger'" size="small">
            {{ state.detail.is_read === 1 ? '已读' : '未读' }}
          </el-tag>
        </el-descriptions-item>
        <el-descriptions-item label="发送时间">{{ formatTime(state.detail.created_at) }}</el-descriptions-item>
        <el-descriptions-item label="已读时间">{{ formatTime(state.detail.read_at) }}</el-descriptions-item>
        <el-descriptions-item label="场景">{{ state.detail.scene || '-' }}</el-descriptions-item>
      </el-descriptions>
      <div class="message-content" v-if="state.detail">{{ state.detail.content || '（无正文）' }}</div>
      <div class="message-param" v-if="state.detail && state.detail.param">
        <div class="message-param__title">场景参数</div>
        <pre>{{ state.detail.param }}</pre>
      </div>
      <template #footer>
        <el-button @click="state.detailVisible = false">关闭</el-button>
        <el-button
            v-if="state.detail && state.detail.is_read === 0"
            type="primary"
            @click="handleMarkRead(state.detail)"
        >标记已读</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script lang="ts" setup name="Message">
import { reactive, ref, onMounted } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import messageApi, { type MessageItem } from '@/api/resource/message'

// 模板适配说明（与 exAdmin 版差异点）：
//   1. 请求封装走模板 utils/request.ts：成功响应解包外层 CommonReply 后直接返回内层 Rsp，
//      故 res 即 {code,msg,total,list|count|updated|data}（无需再取 response.data）；
//      拦截器为 camelCase 字段补 snake_case 别名，行字段仍按 is_read / created_at 直读。
//   2. protojson 将 int64 序列化为字符串：id/时间戳/count/updated 已在使用处 Number() 或
//      模板字面量拼接，天然兼容 string|number。
//   3. 权限：页面按钮不挂 v-auth（菜单/API/casbin 授权数据属权限写入，未在本批落库）。

// 类型字典（与后端 sys_message.type 同口径：1系统 2订单 3结算 4投诉）
const TYPE_MAP: Record<number, string> = { 1: '系统', 2: '订单', 3: '结算', 4: '投诉' }
// Tab → type（all=不过滤）
const TAB_TYPE: Record<string, number | undefined> = {
  all: undefined,
  system: 1,
  order: 2,
  settle: 3,
  complain: 4
}

const typeText = (type?: number) => TYPE_MAP[Number(type || 0)] || '未知'
const typeTag = (type?: number): 'primary' | 'success' | 'warning' | 'info' => {
  switch (Number(type)) {
    case 1:
      return 'primary'
    case 2:
      return 'success'
    case 3:
      return 'warning'
    default:
      return 'info'
  }
}

const formatTime = (ts?: number | string) => {
  const n = Number(ts || 0)
  if (!n) return '-'
  return new Date(n).toLocaleString('zh-CN')
}

const activeType = ref<'all' | 'system' | 'order' | 'settle' | 'complain'>('all')
const unread = ref(0)
const markAllLoading = ref(false)

const state = reactive({
  loading: true,
  total: 0,
  tableData: [] as MessageItem[],
  detailVisible: false,
  detail: null as MessageItem | null
})

const queryParams = reactive({
  page: 1,
  page_size: 10,
  is_read: -1 // -1全部 / 0未读 / 1已读
})

// 未读角标（CountUnread，失败静默：角标非关键路径）
const fetchUnread = async () => {
  try {
    const res = await messageApi.unread()
    unread.value = Number(res?.count || 0)
  } catch (e) {
    unread.value = 0
  }
}

// 列表
const fetchData = async () => {
  state.loading = true
  try {
    const params: any = { page: queryParams.page, page_size: queryParams.page_size, is_read: queryParams.is_read }
    const type = TAB_TYPE[activeType.value]
    if (type !== undefined) params.type = type
    const res = await messageApi.list(params)
    state.tableData = res?.list || []
    state.total = Number(res?.total || 0)
  } catch (e: any) {
    ElMessage.error(e?.message || '获取消息列表失败')
  } finally {
    state.loading = false
  }
}

const handleQuery = () => {
  queryParams.page = 1
  fetchData()
}

const handleTabChange = () => handleQuery()

const handleRefresh = () => {
  fetchData()
  fetchUnread()
}

const handleReset = () => {
  queryParams.is_read = -1
  activeType.value = 'all'
  handleQuery()
}

const handleSizeChange = (size: number) => {
  queryParams.page_size = size
  handleQuery()
}

const handleCurrentChange = (page: number) => {
  queryParams.page = page
  fetchData()
}

// 查看详情：未读 → 自动置已读（服务端幂等）
const handleView = async (row: MessageItem) => {
  try {
    const res = await messageApi.info(row.id)
    state.detail = (res?.data || row) as MessageItem
  } catch (e: any) {
    ElMessage.error(e?.message || '获取消息详情失败')
    return
  }
  state.detailVisible = true
  if (state.detail && state.detail.is_read === 0) {
    try {
      await messageApi.markRead(row.id)
      state.detail.is_read = 1
      fetchUnread()
    } catch (e: any) {
      ElMessage.error(e?.message || '标记已读失败')
    }
  }
}

const handleMarkRead = async (row: MessageItem) => {
  try {
    await messageApi.markRead(row.id)
    if (state.detail && String(state.detail.id) === String(row.id)) state.detail.is_read = 1
    ElMessage.success('已标记为已读')
    fetchUnread()
    fetchData()
  } catch (e: any) {
    ElMessage.error(e?.message || '标记已读失败')
  }
}

const handleMarkAllRead = () => {
  if (unread.value <= 0) {
    ElMessage.info('没有未读消息')
    return
  }
  markAllLoading.value = true
  ElMessageBox({
    message: `是否确认将 ${unread.value} 条未读消息全部标记为已读?`,
    title: '提示',
    showCancelButton: true,
    confirmButtonText: '确定',
    cancelButtonText: '取消',
    type: 'warning'
  })
      .then(async () => {
        try {
          const res = await messageApi.markAllRead()
          ElMessage.success(`已读 ${Number(res?.updated ?? 0)} 条`)
          unread.value = 0
          fetchData()
        } catch (e: any) {
          ElMessage.error(e?.message || '全部已读失败')
        }
      })
      .catch(() => {})
      .finally(() => {
        markAllLoading.value = false
      })
}

const handleDelete = (row: MessageItem) => {
  ElMessageBox({
    message: `是否确认删除消息「${row.title}”?`,
    title: '警告',
    showCancelButton: true,
    confirmButtonText: '确定',
    cancelButtonText: '取消',
    type: 'warning'
  })
      .then(async () => {
        try {
          await messageApi.remove(row.id)
          ElMessage.success('删除成功')
          fetchUnread()
          fetchData()
        } catch (e: any) {
          ElMessage.error(e?.message || '删除失败')
        }
      })
      .catch(() => {})
}

const handleDetailClosed = () => {
  state.detail = null
}

onMounted(() => {
  fetchData()
  fetchUnread()
})
</script>

<style scoped>
.message-container .card-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
}

.header-actions {
  display: flex;
  align-items: center;
  gap: 12px;
}

.unread-badge {
  margin-right: 4px;
}

.message-content {
  margin-top: 16px;
  padding: 12px;
  line-height: 1.8;
  white-space: pre-wrap;
  word-break: break-word;
  background: var(--el-fill-color-light);
  border-radius: 4px;
}

.message-param {
  margin-top: 12px;
}

.message-param__title {
  margin-bottom: 6px;
  color: var(--el-text-color-secondary);
  font-size: 13px;
}

.message-param pre {
  margin: 0;
  padding: 10px;
  overflow: auto;
  font-size: 12px;
  background: var(--el-fill-color-light);
  border-radius: 4px;
}
</style>
