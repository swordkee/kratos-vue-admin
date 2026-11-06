<template>
  <div class="app-container media-file">
    <el-card shadow="always">
      <el-form :inline="true" :model="state.query" class="filter-form">
        <el-form-item label="文件名">
          <el-input v-model="state.query.keyword" placeholder="模糊匹配文件名/备注" clearable style="width: 220px"
                    @keyup.enter="handleQuery"/>
        </el-form-item>
        <el-form-item label="标签">
          <el-select v-model="state.query.tag" placeholder="全部" clearable style="width: 140px">
            <el-option v-for="t in state.tagOptions" :key="t" :label="t" :value="t"/>
          </el-select>
        </el-form-item>
        <el-form-item>
          <el-button type="primary" plain @click="handleQuery">
            <SvgIcon name="elementSearch"/>
            搜索
          </el-button>
          <el-button @click="resetQuery">
            <SvgIcon name="elementRefresh"/>
            重置
          </el-button>
        </el-form-item>
      </el-form>
    </el-card>

    <el-card class="box-card">
      <template #header>
        <div class="card-header">
          <span class="card-header-text">文件管理</span>
          <div class="ops">
            <el-button plain @click="openImport">
              <SvgIcon name="elementLink"/>
              导入URL
            </el-button>
            <el-button type="danger" plain :disabled="!state.ids.length" @click="onBatchDelete">
              <SvgIcon name="elementDelete"/>
              批量删除
            </el-button>
          </div>
        </div>
      </template>
      <el-table v-loading="state.loading" :data="tableData" @selection-change="onSelectionChange">
        <el-table-column type="selection" width="50" align="center"/>
        <el-table-column label="预览" width="90" align="center">
          <template #default="scope">
            <el-image
                v-if="isImage(scope.row)"
                style="width: 52px; height: 52px"
                :src="mediaUrl(scope.row.url)"
                :preview-src-list="[mediaUrl(scope.row.url)]"
                preview-teleported
                fit="cover"
            />
            <span v-else class="muted">—</span>
          </template>
        </el-table-column>
        <el-table-column label="ID" width="80" align="center" prop="id"/>
        <el-table-column label="更新时间" width="170" align="center">
          <template #default="scope">{{ formatMillis(scope.row.updatedAt) }}</template>
        </el-table-column>
        <el-table-column label="文件名/备注" min-width="200">
          <template #default="scope">
            <span class="link-text" @click="onEditName(scope.row)">{{ scope.row.name }}</span>
          </template>
        </el-table-column>
        <el-table-column label="链接" min-width="240">
          <template #default="scope">
            <span class="url-text" :title="scope.row.url">{{ scope.row.url }}</span>
          </template>
        </el-table-column>
        <el-table-column label="标签" width="90" align="center" prop="tag"/>
        <el-table-column label="存储" width="90" align="center" prop="ossType"/>
        <el-table-column label="大小" width="110" align="center">
          <template #default="scope">{{ formatSize(scope.row.size) }}</template>
        </el-table-column>
        <el-table-column label="上传人" width="100" align="center">
          <template #default="scope">{{ scope.row.createdBy || '—' }}</template>
        </el-table-column>
        <el-table-column label="操作" width="160" align="center" fixed="right">
          <template #default="scope">
            <el-button link type="primary" @click="onDownload(scope.row)">下载</el-button>
            <el-button link type="danger" @click="onDelete(scope.row)">删除</el-button>
          </template>
        </el-table-column>
      </el-table>
      <div class="pager">
        <el-pagination
            background
            layout="total, sizes, prev, pager, next, jumper"
            :total="state.total"
            :current-page="state.page"
            :page-size="state.pageSize"
            :page-sizes="[10, 30, 50, 100]"
            @current-change="onPageChange"
            @size-change="onSizeChange"
        />
      </div>
    </el-card>

    <el-dialog v-model="state.importVisible" title="导入URL" width="560px">
      <el-input
          v-model="state.importText"
          type="textarea"
          :rows="8"
          placeholder="我的图片|https://my-oss.com/my.png&#10;https://my-oss.com/my_1.png"
      />
      <template #footer>
        <el-button @click="state.importVisible = false">取 消</el-button>
        <el-button type="primary" :loading="state.submitting" @click="submitImport">确 定</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script lang="ts" setup>
import { computed, onMounted, reactive } from 'vue';
import { ElMessage, ElMessageBox } from 'element-plus';
import {
  deleteFile,
  deleteFiles,
  downloadFile,
  editFileName,
  getFileList,
  importURL,
  mediaUrl,
  saveBlob,
} from '@/api/media/file';
import { formatMillis } from '@/utils/media';

defineOptions({ name: 'mediaFile' });

const state = reactive({
  loading: false,
  submitting: false,
  ids: [] as any[],
  total: 0,
  page: 1,
  pageSize: 10,
  rows: [] as any[],
  query: { keyword: '', tag: '' },
  tagOptions: ['png', 'jpg', 'jpeg', 'gif', 'webp', 'bmp', 'svg', 'mp4', 'pdf'],
  importVisible: false,
  importText: '',
});

// 标签过滤在前端作用于当前页（后端列表只支持 keyword，源端同口径）
const tableData = computed(() => {
  const tag = (state.query.tag || '').toLowerCase();
  if (!tag) return state.rows;
  return state.rows.filter((r: any) => String(r.tag || '').toLowerCase() === tag);
});

const isImage = (row: any) => ['png', 'jpg', 'jpeg', 'gif', 'webp', 'bmp'].includes(String(row.tag || '').toLowerCase());
const formatSize = (v: any) => {
  const n = Number(v || 0);
  if (!n) return '—';
  if (n < 1024) return `${n} B`;
  if (n < 1024 * 1024) return `${(n / 1024).toFixed(1)} KB`;
  return `${(n / 1024 / 1024).toFixed(2)} MB`;
};

const fetchList = () => {
  state.loading = true;
  getFileList({ page: state.page, pageSize: state.pageSize, keyword: state.query.keyword })
      .then((res: any) => {
        state.rows = res.list || [];
        state.total = Number(res.total || 0);
      })
      .catch(() => {
        state.rows = [];
        state.total = 0;
      })
      .finally(() => {
        state.loading = false;
      });
};

const handleQuery = () => {
  state.page = 1;
  fetchList();
};

const resetQuery = () => {
  state.query.keyword = '';
  state.query.tag = '';
  handleQuery();
};

const onPageChange = (p: number) => {
  state.page = p;
  fetchList();
};

const onSizeChange = (s: number) => {
  state.pageSize = s;
  state.page = 1;
  fetchList();
};

const onSelectionChange = (rows: any[]) => {
  state.ids = rows.map((r) => r.id);
};

const onEditName = (row: any) => {
  ElMessageBox.prompt('请输入文件名或者备注', '编辑', {
    confirmButtonText: '确定',
    cancelButtonText: '取消',
    inputValue: row.name,
    inputPattern: /\S/,
    inputErrorMessage: '不能为空',
  })
      .then(({ value }: any) => {
        editFileName({ id: row.id, name: value })
            .then(() => {
              ElMessage.success('编辑成功');
              fetchList();
            })
            .catch(() => undefined);
      })
      .catch(() => undefined);
};

const onDelete = (row: any) => {
  ElMessageBox.confirm('此操作将永久删除文件, 是否继续?', '提示', { type: 'warning' })
      .then(() => {
        deleteFile(row.id)
            .then(() => {
              ElMessage.success('删除成功');
              if (state.rows.length === 1 && state.page > 1) state.page -= 1;
              fetchList();
            })
            .catch(() => undefined);
      })
      .catch(() => undefined);
};

const onBatchDelete = () => {
  ElMessageBox.confirm(`确认删除勾选的 ${state.ids.length} 个文件？`, '提示', { type: 'warning' })
      .then(() => {
        deleteFiles(state.ids)
            .then((res: any) => {
              const failed = res.failedIds || [];
              if (failed.length) ElMessage.warning(`部分删除失败：${failed.join(',')}`);
              else ElMessage.success('删除成功');
              fetchList();
            })
            .catch(() => undefined);
      })
      .catch(() => undefined);
};

const onDownload = (row: any) => {
  downloadFile(row.id)
      .then((res: any) => saveBlob(new Blob([res.data]), row.name || 'download'))
      .catch(() => ElMessage.error('下载失败'));
};

const openImport = () => {
  state.importText = '';
  state.importVisible = true;
};

const submitImport = () => {
  const lines = (state.importText || '').split('\n').map((l) => l.trim()).filter((l) => l);
  if (!lines.length) {
    ElMessage.warning('请输入至少一条链接');
    return;
  }
  const list = lines.map((line) => {
    const seg = line.split('|');
    const url = (seg.length > 1 ? seg[1] : seg[0]).trim();
    const name = (seg.length > 1 ? seg[0] : url.substring(url.lastIndexOf('/') + 1)).trim();
    const dot = url.lastIndexOf('.');
    return { name, url, tag: dot > 0 ? url.substring(dot + 1) : '', key: '' };
  });
  state.submitting = true;
  importURL(list)
      .then((res: any) => {
        ElMessage.success(`导入成功（${res.count || list.length} 条）`);
        state.importVisible = false;
        fetchList();
      })
      .catch(() => undefined)
      .finally(() => {
        state.submitting = false;
      });
};

onMounted(fetchList);
</script>

<style scoped lang="scss">
.media-file {
  .filter-form {
    margin-bottom: -18px;
  }

  .box-card {
    margin-top: 12px;
  }

  .card-header {
    display: flex;
    align-items: center;
    justify-content: space-between;
  }

  .card-header-text {
    font-weight: 600;
  }

  .link-text {
    color: #409eff;
    cursor: pointer;
  }

  .url-text {
    display: inline-block;
    max-width: 100%;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
    color: #606266;
  }

  .muted {
    color: #909399;
    font-size: 12px;
  }

  .pager {
    display: flex;
    justify-content: flex-end;
    padding-top: 12px;
  }
}
</style>
