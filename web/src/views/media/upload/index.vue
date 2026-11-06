<template>
  <div class="app-container media-upload">
    <!-- 左侧：分类树（源 upload.vue 左栏 el-tree + 下拉增删改） -->
    <el-card shadow="always" class="cate-card">
      <template #header>
        <div class="card-header">
          <span class="card-header-text">媒体分类</span>
          <el-button type="primary" text @click="openCategory('create', { id: 0 })">
            <SvgIcon name="elementPlus"/>
          </el-button>
        </div>
      </template>
      <el-scrollbar height="calc(100vh - 260px)">
        <el-tree
            :data="state.categories"
            node-key="id"
            :props="{ label: 'name', children: 'children' }"
            default-expand-all
            highlight-current
            @node-click="handleNodeClick"
        >
          <template #default="{ node, data }">
            <div class="tree-node">
              <span :class="{ 'node-active': state.activePid === data.id }">{{ data.name }}</span>
              <el-dropdown trigger="click" @command="(cmd) => onNodeCommand(cmd, data)">
                <el-button text size="small">
                  <SvgIcon name="elementMoreFilled"/>
                </el-button>
                <template #dropdown>
                  <el-dropdown-menu>
                    <el-dropdown-item command="add">添加分类</el-dropdown-item>
                    <el-dropdown-item command="edit" v-if="data.id > 0">编辑分类</el-dropdown-item>
                    <el-dropdown-item command="delete" divided v-if="data.id > 0">删除分类</el-dropdown-item>
                  </el-dropdown-menu>
                </template>
              </el-dropdown>
            </div>
          </template>
        </el-tree>
      </el-scrollbar>
    </el-card>

    <div class="media-main">
      <el-card shadow="always">
        <div class="toolbar">
          <!-- 多文件上传：源 upload-common / upload-image / cropper 三个入口合并为一个（默认多选、显示每文件进度） -->
          <el-upload
              v-model:file-list="state.uploadFiles"
              :action="actionUrl"
              :headers="headers"
              :data="{ classId: state.activePid }"
              multiple
              :show-file-list="true"
              :on-success="onUploadSuccess"
              :on-error="onUploadError"
              :on-progress="onUploadProgress"
              :on-exceed="onUploadExceed"
              :limit="20"
              class="inline-upload"
          >
            <el-button type="primary" plain>
              <SvgIcon name="elementUpload"/>
              上传文件
            </el-button>
          </el-upload>
          <el-button plain @click="state.chunkVisible = true">
            <SvgIcon name="elementFiles"/>
            大文件分片上传
          </el-button>
          <el-button plain @click="openScan">
            <SvgIcon name="elementIphone"/>
            扫码上传
          </el-button>
          <el-button plain @click="openImport">
            <SvgIcon name="elementLink"/>
            导入URL
          </el-button>
          <el-button type="danger" plain :disabled="!state.ids.length" @click="onBatchDelete">
            <SvgIcon name="elementDelete"/>
            批量删除
          </el-button>
          <el-input v-model="state.keyword" class="kw" placeholder="请输入文件名或备注" clearable
                    @keyup.enter="handleQuery" style="width: 220px"/>
          <el-button type="primary" plain @click="handleQuery">
            <SvgIcon name="elementSearch"/>
            查询
          </el-button>
        </div>
      </el-card>

      <el-card class="box-card">
        <template #header>
          <div class="card-header">
            <span class="card-header-text">
              文件列表{{ state.activePid > 0 ? '（当前分类：' + activeCateName() + '，仅作上传归类标记）' : '' }}
            </span>
            <span class="tip">点击文件名可改名；下载走后端流式接口</span>
          </div>
        </template>
        <el-table v-loading="state.loading" :data="state.tableData" @selection-change="onSelectionChange">
          <el-table-column type="selection" width="50" align="center"/>
          <el-table-column label="预览" width="100" align="center">
            <template #default="scope">
              <el-image
                  v-if="isImage(scope.row)"
                  style="width: 56px; height: 56px"
                  :src="mediaUrl(scope.row.url)"
                  :preview-src-list="[mediaUrl(scope.row.url)]"
                  preview-teleported
                  fit="cover"
              />
              <span v-else class="muted">—</span>
            </template>
          </el-table-column>
          <el-table-column label="日期" width="170" align="center">
            <template #default="scope">{{ formatMillis(scope.row.updatedAt) }}</template>
          </el-table-column>
          <el-table-column label="文件名/备注" min-width="200">
            <template #default="scope">
              <span class="link-text" @click="onEditName(scope.row)">{{ scope.row.name }}</span>
            </template>
          </el-table-column>
          <el-table-column label="链接" min-width="260">
            <template #default="scope">
              <span class="url-text" :title="scope.row.url" @click="copyUrl(scope.row.url)">{{ scope.row.url }}</span>
            </template>
          </el-table-column>
          <el-table-column label="标签" width="90" align="center">
            <template #default="scope">
              <el-tag disable-transitions :type="scope.row.tag === 'jpg' ? 'info' : 'success'">
                {{ scope.row.tag }}
              </el-tag>
            </template>
          </el-table-column>
          <el-table-column label="分辨率" width="110" align="center" prop="resolution"/>
          <el-table-column label="大小" width="110" align="center">
            <template #default="scope">{{ formatSize(scope.row.size) }}</template>
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
    </div>

    <!-- 分类弹窗（源 categoryDialogVisible：上级分类 + 名称） -->
    <el-dialog v-model="state.categoryVisible" :title="state.categoryTitle" width="520px" destroy-on-close>
      <el-form ref="categoryFormRef" :model="state.categoryForm" :rules="state.categoryRules" label-width="90px">
        <el-form-item label="上级分类">
          <el-tree-select
              v-model="state.categoryForm.pid"
              :data="state.categoryTreeSelect"
              check-strictly
              :props="{ label: 'name', children: 'children', value: 'id' }"
              :render-after-expand="false"
              style="width: 240px"
          />
        </el-form-item>
        <el-form-item label="分类名称" prop="name">
          <el-input v-model.trim="state.categoryForm.name" placeholder="分类名称" maxlength="20" show-word-limit/>
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="state.categoryVisible = false">取 消</el-button>
        <el-button type="primary" :loading="state.submitting" @click="submitCategory">确 定</el-button>
      </template>
    </el-dialog>

    <!-- 大文件分片上传（源 breakpoint.vue：1MB 分片 + 秒传/续传/合并/清片） -->
    <el-dialog v-model="state.chunkVisible" title="大文件分片上传" width="620px" @closed="resetChunk">
      <el-form label-width="90px">
        <el-form-item label="选择文件">
          <input type="file" @change="onChunkFileChange"/>
        </el-form-item>
        <el-form-item label="进度">
          <el-progress :percentage="state.chunkPercent" :status="state.chunkStatus" style="width: 100%"/>
        </el-form-item>
        <el-form-item label="说明">
          <span class="muted">
            分片 1MB；同指纹文件再次上传自动秒传；中途中断重选同一文件会跳过已上传分片。
          </span>
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="state.chunkVisible = false">关 闭</el-button>
        <el-button type="primary" :loading="state.chunkRunning" :disabled="!state.chunkFile" @click="startChunkUpload">
          开始上传
        </el-button>
      </template>
    </el-dialog>

    <!-- 扫码上传（源 QR-code.vue + scanUpload.vue：手机端扫码直达 /scanUpload） -->
    <el-dialog v-model="state.scanVisible" title="扫码上传" width="360px">
      <div class="scan-body">
        <TwoDimensionalCode v-if="state.scanUrl" :rule-form="{ qrcode: state.scanUrl }" :width="220" :height="220"/>
        <p class="muted">手机扫描二维码即可拍照上传（token 有效期同登录态）</p>
        <p class="scan-url">{{ state.scanUrl }}</p>
      </div>
    </el-dialog>

    <!-- 导入 URL（源 importURL：每行「名称|链接」或仅链接） -->
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
import { computed, onMounted, reactive, ref } from 'vue';
import { ElMessage, ElMessageBox } from 'element-plus';
import { addCategory, deleteCategory, getCategoryList } from '@/api/media/category';
import {
  breakpointContinue,
  breakpointContinueFinish,
  deleteFile,
  deleteFiles,
  downloadFile,
  editFileName,
  findFile,
  getFileList,
  importURL,
  mediaAbsoluteUrl,
  mediaUrl,
  removeChunk,
  saveBlob,
  uploadAction,
  uploadHeaders,
} from '@/api/media/file';
import { formatMillis } from '@/utils/media';
import { md5ArrayBuffer, md5Blob } from '@/utils/md5';
import { Session } from '@/utils/storage';
import TwoDimensionalCode from '@/components/twoDimensionalCode/index.vue';

defineOptions({ name: 'mediaUpload' });

const CHUNK_SIZE = 1024 * 1024;

const categoryFormRef = ref();
const actionUrl = uploadAction();
const headers = uploadHeaders();

const state = reactive({
  loading: false,
  submitting: false,
  ids: [] as any[],
  keyword: '',
  page: 1,
  pageSize: 10,
  total: 0,
  tableData: [] as any[],
  categories: [] as any[],
  activePid: 0,
  uploadFiles: [] as any[],
  // 分类弹窗
  categoryVisible: false,
  categoryTitle: '',
  categoryId: 0,
  categoryForm: { id: 0, name: '', pid: 0 } as any,
  categoryRules: {
    name: [
      { required: true, message: '请输入分类名称', trigger: 'blur' },
      { max: 20, message: '最多20位字符', trigger: 'blur' },
    ],
  },
  // 分片
  chunkVisible: false,
  chunkFile: null as File | null,
  chunkPercent: 0,
  chunkStatus: '' as '' | 'success' | 'exception' | 'warning',
  chunkRunning: false,
  // 扫码
  scanVisible: false,
  scanUrl: '',
  // 导入
  importVisible: false,
  importText: '',
});

// 分类树新增/编辑用（0 视为根，供 tree-select 选根）
const categoryTreeSelect = computed(() => [
  { id: 0, name: '根分类', children: state.categories },
]);

const isImage = (row: any) => ['png', 'jpg', 'jpeg', 'gif', 'webp', 'bmp'].includes(String(row.tag || '').toLowerCase());
const formatSize = (v: any) => {
  const n = Number(v || 0);
  if (!n) return '—';
  if (n < 1024) return `${n} B`;
  if (n < 1024 * 1024) return `${(n / 1024).toFixed(1)} KB`;
  return `${(n / 1024 / 1024).toFixed(2)} MB`;
};
const activeCateName = () => {
  const hit = findCate(state.categories, state.activePid);
  return hit ? hit.name : '';
};
const findCate = (list: any[], id: number): any => {
  for (const c of list) {
    if (Number(c.id) === Number(id)) return c;
    if (c.children && c.children.length) {
      const hit = findCate(c.children, id);
      if (hit) return hit;
    }
  }
  return null;
};

const handleQuery = () => {
  state.page = 1;
  fetchList();
};

const fetchList = () => {
  state.loading = true;
  getFileList({ page: state.page, pageSize: state.pageSize, keyword: state.keyword })
      .then((res: any) => {
        state.tableData = res.list || [];
        state.total = Number(res.total || 0);
      })
      .catch(() => {
        state.tableData = [];
        state.total = 0;
      })
      .finally(() => {
        state.loading = false;
      });
};

const fetchCategories = () => {
  getCategoryList()
      .then((res: any) => {
        state.categories = res.list || [];
      })
      .catch(() => {
        state.categories = [];
      });
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

const handleNodeClick = (node: any) => {
  // 源端 classId 只用于上传归类标记（wp 表结构无 class 列），不参与列表过滤
  state.activePid = Number(node.id || 0);
};

const onNodeCommand = (cmd: string, node: any) => {
  if (cmd === 'add') openCategory('create', { id: node.id });
  if (cmd === 'edit') openCategory('edit', node);
  if (cmd === 'delete') onDeleteCategory(node.id);
};

const openCategory = (mode: string, node: any) => {
  state.categoryTitle = mode === 'create' ? '添加分类' : '编辑分类';
  state.categoryForm = mode === 'create'
      ? { id: 0, name: '', pid: Number(node.id || 0) }
      : { id: Number(node.id), name: node.name, pid: Number(node.pid || 0) };
  state.categoryVisible = true;
};

const submitCategory = () => {
  categoryFormRef.value?.validate((valid: boolean) => {
    if (!valid) return;
    state.submitting = true;
    addCategory(state.categoryForm)
        .then(() => {
          ElMessage.success('操作成功');
          state.categoryVisible = false;
          fetchCategories();
        })
        .catch(() => undefined)
        .finally(() => {
          state.submitting = false;
        });
  });
};

const onDeleteCategory = (id: number) => {
  ElMessageBox.confirm('确认删除该分类？有子级时会被拒绝', '提示', { type: 'warning' })
      .then(() => {
        deleteCategory(id)
            .then(() => {
              ElMessage.success('删除成功');
              fetchCategories();
            })
            .catch(() => undefined);
      })
      .catch(() => undefined);
};

// ---- 上传（el-upload 直连 action，逐文件进度由组件展示）----
const onUploadProgress = () => undefined;
const onUploadSuccess = (res: any) => {
  if (res && res.code === 200) {
    ElMessage.success('上传成功');
    state.uploadFiles = [];
    fetchList();
  } else {
    ElMessage.error('上传失败');
  }
};
const onUploadError = (err: any) => {
  let msg = '上传失败';
  try {
    const body = JSON.parse(err.message || '{}');
    msg = body.message || msg;
  } catch (e) {
    msg = err.message || msg;
  }
  ElMessage.error(msg);
};
const onUploadExceed = () => ElMessage.warning('单次最多 20 个文件');

// ---- 改名 / 删除 / 下载 ----
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
              if (state.tableData.length === 1 && state.page > 1) state.page -= 1;
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
      .then((res: any) => {
        saveBlob(new Blob([res.data]), row.name || 'download');
      })
      .catch(() => ElMessage.error('下载失败'));
};

const copyUrl = (url: string) => {
  navigator.clipboard?.writeText(mediaAbsoluteUrl(url)).then(() => ElMessage.success('链接已复制')).catch(() => undefined);
};

// ---- 分片上传 ----
const onChunkFileChange = (e: any) => {
  state.chunkFile = e.target.files?.[0] || null;
  state.chunkPercent = 0;
  state.chunkStatus = '';
};

const resetChunk = () => {
  state.chunkFile = null;
  state.chunkPercent = 0;
  state.chunkStatus = '';
  state.chunkRunning = false;
};

// 整文件指纹：真实内容 MD5（后端 Finish 会复算合并结果并比对，不符即拒绝、整体重传）。
// 浏览器无内置 MD5 ⇒ 用 utils/md5 的纯 TS 实现；分片 md5 同一口径（双口径 sha256: 仍被后端支持）。
const fileFingerprint = (file: File) => md5Blob(file);

const startChunkUpload = async () => {
  const file = state.chunkFile;
  if (!file) return;
  state.chunkRunning = true;
  state.chunkStatus = '';
  try {
    const fileMd5 = await fileFingerprint(file);
    const chunkTotal = Math.ceil(file.size / CHUNK_SIZE) || 1;
    const probe: any = await findFile({ fileMd5, fileName: file.name, chunkTotal });
    const fileInfo = probe.file || {};
    if (fileInfo.isFinish) {
      state.chunkPercent = 100;
      state.chunkStatus = 'success';
      ElMessage.success('文件已秒传');
      fetchList();
      return;
    }
    const done = new Set<number>((fileInfo.chunks || []).map((c: any) => Number(c.fileChunkNumber)));
    for (let i = 0; i < chunkTotal; i += 1) {
      const slice = file.slice(i * CHUNK_SIZE, (i + 1) * CHUNK_SIZE);
      if (done.has(i)) {
        state.chunkPercent = Math.floor(((i + 1) / chunkTotal) * 100);
        continue;
      }
      const buf = await slice.arrayBuffer();
      const form = new FormData();
      form.append('fileMd5', fileMd5);
      form.append('fileName', file.name);
      form.append('chunkNumber', String(i));
      form.append('chunkTotal', String(chunkTotal));
      form.append('chunkMd5', md5ArrayBuffer(buf));
      form.append('file', new Blob([buf]), `${file.name}.part`);
      await breakpointContinue(form);
      state.chunkPercent = Math.floor(((i + 1) / chunkTotal) * 100);
    }
    const merged: any = await breakpointContinueFinish({ fileMd5, fileName: file.name });
    await removeChunk({ fileMd5, filePath: merged.filePath });
    state.chunkPercent = 100;
    state.chunkStatus = 'success';
    ElMessage.success('分片上传完成');
    fetchList();
  } catch (e: any) {
    state.chunkStatus = 'exception';
    ElMessage.error(e?.message || '分片上传失败');
  } finally {
    state.chunkRunning = false;
  }
};

// ---- 扫码上传 ----
const openScan = () => {
  const token = Session.get('token') || '';
  const base = `${window.location.protocol}//${window.location.host}${window.location.pathname}`;
  state.scanUrl = `${base}#/scanUpload?pid=${state.activePid}&token=${encodeURIComponent(token)}`;
  state.scanVisible = true;
};

// ---- 导入 URL ----
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

onMounted(() => {
  fetchCategories();
  fetchList();
});
</script>

<style scoped lang="scss">
.media-upload {
  display: flex;
  gap: 12px;
  align-items: flex-start;

  .cate-card {
    flex: none;
    width: 240px;
  }

  .media-main {
    flex: 1;
    min-width: 0;

    .box-card {
      margin-top: 12px;
    }
  }

  .toolbar {
    display: flex;
    flex-wrap: wrap;
    gap: 10px;
    align-items: center;
  }

  .inline-upload {
    display: inline-block;
  }

  .card-header {
    display: flex;
    align-items: center;
    justify-content: space-between;
  }

  .card-header-text {
    font-weight: 600;
  }

  .tree-node {
    display: flex;
    align-items: center;
    justify-content: space-between;
    width: 100%;
  }

  .node-active {
    color: #409eff;
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
    cursor: pointer;
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

  .scan-body {
    display: flex;
    flex-direction: column;
    align-items: center;
    gap: 8px;

    .scan-url {
      word-break: break-all;
      font-size: 12px;
      color: #909399;
    }
  }
}
</style>
