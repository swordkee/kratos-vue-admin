<template>
  <el-dialog v-model="dialogVisible" title="二因素认证（TOTP）" width="520px" @open="onOpen" :close-on-click-modal="false">
    <!-- 状态查询中 -->
    <div v-if="loading" class="mfa-dialog-loading">
      <el-skeleton :rows="3" animated />
    </div>

    <!-- 未绑定：绑定流程 -->
    <template v-else-if="!statusObj.enabled">
      <!-- 步骤 1：申请密钥 -->
      <template v-if="!enrollInfo.url">
        <el-empty description="尚未绑定二因素认证" :image-size="90" />
        <div class="mfa-dialog-tip">
          绑定后登录需输入 6 位动态验证码，账号更安全。请先在手机安装 Google Authenticator / 1Password 等 TOTP 应用。
        </div>
        <div class="mfa-dialog-actions">
          <el-button type="primary" @click="onEnroll" :loading="enrollLoading">立即绑定</el-button>
        </div>
      </template>
      <!-- 步骤 2：扫码 + 输入动态码启用 -->
      <template v-else>
        <div class="mfa-dialog-qr">
          <twoDimensionalCode :rule-form="{ qrcode: enrollInfo.url }" :width="180" :height="180" />
        </div>
        <div class="mfa-dialog-secret">
          <span>无法扫码？手动输入密钥：</span>
          <el-tag>{{ enrollInfo.secret }}</el-tag>
          <el-button size="small" link type="primary" @click="copySecret">复制</el-button>
        </div>
        <el-input v-model="code" placeholder="输入应用显示的 6 位动态码" maxlength="6" clearable class="mt15" />
        <div class="mfa-dialog-actions">
          <el-button type="primary" @click="onEnable" :loading="enableLoading">确认绑定</el-button>
          <el-button @click="resetEnroll">重新生成</el-button>
        </div>
      </template>
    </template>

    <!-- 已绑定：查看状态 + 关闭 -->
    <template v-else>
      <el-descriptions :column="1" border class="mt15">
        <el-descriptions-item label="状态">
          <el-tag type="success">已开启</el-tag>
        </el-descriptions-item>
        <el-descriptions-item label="绑定时间">{{ formatBoundAt(statusObj.boundAt) }}</el-descriptions-item>
      </el-descriptions>
      <el-input v-model="code" placeholder="输入动态码或恢复码以关闭" maxlength="8" clearable class="mt15" />
      <div class="mfa-dialog-actions">
        <el-button type="danger" @click="onDisable" :loading="disableLoading">关闭二因素</el-button>
      </div>
    </template>

    <!-- 恢复码展示（仅绑定成功时出现一次） -->
    <div v-if="recoveryCodes.length" class="mfa-dialog-codes-wrap">
      <el-alert type="warning" :closable="false" show-icon title="请妥善保存以下 10 个恢复码（仅展示这次，丢失需关闭后重新绑定）">
        <div class="mfa-dialog-codes">
          <el-tag v-for="(c, i) in recoveryCodes" :key="i" type="warning" effect="plain" class="mfa-dialog-code">{{ c }}</el-tag>
        </div>
        <div class="mfa-dialog-export">
          <el-button size="small" type="primary" plain @click="exportRecoveryCodes">导出为 TXT 文件</el-button>
        </div>
      </el-alert>
    </div>
  </el-dialog>
</template>

<script setup lang="ts">
import { ref, computed, getCurrentInstance } from 'vue';
import { ElMessage } from 'element-plus';
import { mfaStatus, mfaEnroll, mfaEnable, mfaDisable } from '@/api/mfa';
import twoDimensionalCode from '@/components/twoDimensionalCode/index.vue';

const { proxy } = getCurrentInstance() as any;

const props = defineProps({
  modelValue: { type: Boolean, default: false },
  username: { type: String, default: '' },
});
const emit = defineEmits(['update:modelValue']);

const dialogVisible = computed({
  get: () => props.modelValue,
  set: (v: boolean) => emit('update:modelValue', v),
});

const loading = ref(false);
const enrollLoading = ref(false);
const enableLoading = ref(false);
const disableLoading = ref(false);
const statusObj = ref<{ available: boolean; enabled: boolean; boundAt: number }>({
  available: false,
  enabled: false,
  boundAt: 0,
});
const enrollInfo = ref<{ secret: string; url: string }>({ secret: '', url: '' });
const code = ref('');
const recoveryCodes = ref<string[]>([]);

// 打开弹窗：查询当前用户状态
const onOpen = async () => {
  code.value = '';
  recoveryCodes.value = [];
  enrollInfo.value = { secret: '', url: '' };
  loading.value = true;
  try {
    const res: any = await mfaStatus();
    // request 成功时已解包 data；兼容历史上返回 {data:{...}} 包装的形态
    statusObj.value = res ?? { available: false, enabled: false, boundAt: 0 };
  } catch (e) {
    statusObj.value = { available: false, enabled: false, boundAt: 0 };
  } finally {
    loading.value = false;
  }
};

// 步骤 1：申请密钥（otpauth URI）
const onEnroll = async () => {
  enrollLoading.value = true;
  try {
    const res: any = await mfaEnroll(props.username);
    enrollInfo.value = res ?? { secret: '', url: '' };
  } finally {
    enrollLoading.value = false;
  }
};

// 步骤 2：校验动态码并启用，返回恢复码
const onEnable = async () => {
  if (!code.value) {
    ElMessage.warning('请输入动态验证码');
    return;
  }
  enableLoading.value = true;
  try {
    const res: any = await mfaEnable(code.value.trim());
    recoveryCodes.value = res?.recoveryCodes ?? [];
    statusObj.value.enabled = true;
    code.value = '';
    ElMessage.success('绑定成功，请妥善保存恢复码');
  } catch (e) {
    code.value = '';
  } finally {
    enableLoading.value = false;
  }
};

// 关闭 TOTP（动态码或恢复码二次确认）
const onDisable = async () => {
  if (!code.value) {
    ElMessage.warning('请输入动态码或恢复码');
    return;
  }
  disableLoading.value = true;
  try {
    await mfaDisable(code.value.trim());
    statusObj.value.enabled = false;
    statusObj.value.boundAt = 0;
    code.value = '';
    ElMessage.success('二因素认证已关闭');
  } catch (e) {
    code.value = '';
  } finally {
    disableLoading.value = false;
  }
};

// 重新生成密钥（重置本地步骤，再次点击立即绑定会重新申请）
const resetEnroll = () => {
  enrollInfo.value = { secret: '', url: '' };
  code.value = '';
};

// 复制密钥（利用选中的 el-tag 文本 fallback，clipboard 失败时提示手动复制）
const copySecret = async () => {
  try {
    await navigator.clipboard.writeText(enrollInfo.value.secret);
    ElMessage.success('密钥已复制');
  } catch (e) {
    ElMessage.info(`请手动复制密钥：${enrollInfo.value.secret}`);
  }
};

// 导出恢复码为 TXT（带 UTF-8 BOM，Windows 记事本可正确识别中文）
const exportRecoveryCodes = () => {
  if (!recoveryCodes.value.length) return;
  const now = new Date();
  const pad = (n: number) => String(n).padStart(2, '0');
  const ts = `${now.getFullYear()}-${pad(now.getMonth() + 1)}-${pad(now.getDate())} ${pad(now.getHours())}:${pad(now.getMinutes())}:${pad(now.getSeconds())}`;
  const stamp = `${now.getFullYear()}${pad(now.getMonth() + 1)}${pad(now.getDate())}-${pad(now.getHours())}${pad(now.getMinutes())}${pad(now.getSeconds())}`;
  const lines = [
    'BMALL 二因素认证（TOTP）恢复码',
    '==============================',
    `账号：${props.username || '-'}`,
    `生成时间：${ts}`,
    '用途：无法使用验证器 App 时，可用恢复码完成登录二次验证或关闭二因素认证（每个恢复码仅可使用一次）',
    '',
    '恢复码：',
    ...recoveryCodes.value.map((c, i) => `${String(i + 1).padStart(2, ' ')}. ${c}`),
    '',
    '警告：请离线妥善保管本文件；一旦泄露，请立即关闭并重新绑定二因素认证。',
    '',
  ];
  const blob = new Blob(['﻿' + lines.join('\r\n')], { type: 'text/plain;charset=utf-8' });
  const url = URL.createObjectURL(blob);
  const a = document.createElement('a');
  a.href = url;
  a.download = `bmall-mfa-recovery-codes-${stamp}.txt`;
  document.body.appendChild(a);
  a.click();
  document.body.removeChild(a);
  URL.revokeObjectURL(url);
  ElMessage.success('恢复码已导出');
};

const formatBoundAt = (ts: number) => {
  if (!ts) return '-';
  const d = new Date(ts);
  const pad = (n: number) => String(n).padStart(2, '0');
  return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())} ${pad(d.getHours())}:${pad(d.getMinutes())}`;
};
</script>

<style scoped lang="scss">
.mfa-dialog-loading {
  padding: 10px 0;
}
.mfa-dialog-tip {
  color: var(--el-text-color-secondary);
  font-size: 13px;
  line-height: 1.6;
  margin: 10px 0 6px;
}
.mfa-dialog-actions {
  margin-top: 15px;
  display: flex;
  gap: 10px;
}
.mfa-dialog-qr {
  display: flex;
  justify-content: center;
  padding: 10px 0;
}
.mfa-dialog-secret {
  display: flex;
  align-items: center;
  gap: 8px;
  font-size: 13px;
  margin-bottom: 10px;
  flex-wrap: wrap;
}
.mfa-dialog-codes-wrap {
  margin-top: 15px;
}
.mfa-dialog-codes {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
  margin-top: 8px;
}
.mfa-dialog-code {
  font-family: monospace;
}
.mfa-dialog-export {
  margin-top: 10px;
  display: flex;
  justify-content: flex-end;
}
</style>
