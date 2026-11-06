<template>
  <div class="scan-upload">
    <div class="header">
      <span class="title">扫码上传</span>
      <span class="hint">选择图片后直接上传；支持多选</span>
    </div>

    <el-upload
        ref="uploadRef"
        class="h5-uploader"
        :action="actionUrl"
        :headers="headers"
        :data="{ classId: state.pid }"
        accept="image/*"
        multiple
        :show-file-list="true"
        :on-success="onSuccess"
        :on-error="onError"
    >
      <div class="upload-box">
        <SvgIcon name="elementPlus"/>
        <span>点击选择图片</span>
      </div>
    </el-upload>

    <div class="footer">
      <el-button type="primary" :loading="state.loading" @click="submit">上传</el-button>
      <span class="tip">上传成功后可在「媒体库 → 文件管理」查看</span>
    </div>
  </div>
</template>

<script lang="ts" setup>
import { computed, onMounted, reactive, ref } from 'vue';
import { useRoute } from 'vue-router';
import { ElMessage } from 'element-plus';
import { uploadAction, uploadHeaders } from '@/api/media/file';

// 手机扫码直达页（源 view/example/upload/scanUpload.vue）：
//   - 路由为静态路由 /scanUpload（不进侧边菜单）；
//   - token 由二维码链接携带（router/index.ts 的守卫把它写进 Session 后放行），
//     因此这里只需要普通上传 action + Authorization 头。
defineOptions({ name: 'mediaScanUpload' });

const route = useRoute();
const uploadRef = ref();
const actionUrl = uploadAction();
const headers = computed(() => uploadHeaders());

const state = reactive({
  pid: 0,
  loading: false,
  failed: false,
});

const onSuccess = (res: any) => {
  if (res && res.code === 200) {
    ElMessage.success('上传成功');
  } else {
    state.failed = true;
    ElMessage.error('上传失败');
  }
};

const onError = (err: any) => {
  state.failed = true;
  let msg = '上传失败';
  try {
    const body = JSON.parse(err.message || '{}');
    msg = body.message || msg;
  } catch (e) {
    msg = err.message || msg;
  }
  ElMessage.error(msg);
};

const submit = () => {
  state.loading = true;
  uploadRef.value?.submit?.();
  setTimeout(() => {
    state.loading = false;
  }, 800);
};

onMounted(() => {
  const pid = Number(route.query.pid || 0);
  state.pid = Number.isFinite(pid) ? pid : 0;
});
</script>

<style scoped lang="scss">
.scan-upload {
  min-height: 100vh;
  padding: 16px;
  background: #f5f7fa;

  .header {
    display: flex;
    flex-direction: column;
    gap: 4px;
    margin-bottom: 16px;

    .title {
      font-size: 18px;
      font-weight: 600;
    }

    .hint {
      font-size: 12px;
      color: #909399;
    }
  }

  .upload-box {
    display: flex;
    flex-direction: column;
    align-items: center;
    justify-content: center;
    gap: 8px;
    width: 100%;
    height: 140px;
    border: 1px dashed #dcdfe6;
    border-radius: 8px;
    background: #fff;
    color: #909399;
  }

  .footer {
    margin-top: 16px;
    display: flex;
    flex-direction: column;
    gap: 8px;

    .tip {
      font-size: 12px;
      color: #909399;
      text-align: center;
    }
  }

  :deep(.h5-uploader) {
    display: block;
    width: 100%;
  }
}
</style>
