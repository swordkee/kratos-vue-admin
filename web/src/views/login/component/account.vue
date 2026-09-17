<template>
  <!-- TOTP：未触发二因素时显示账号密码登录表单 -->
  <el-form v-if="!state.isMfaStep" ref="loginFormRef" size="large" :model="state.loginForm" :rules="state.rules" class="login-content-form">
    <el-form-item class="login-animation-one">
      <el-input type="text" :placeholder="$t('message.account.accountPlaceholder1')" v-model="state.loginForm.username"
        clearable autocomplete="off">
        <template #prefix>
          <el-icon class="el-input__icon">
            <elementUser />
          </el-icon>
        </template>
      </el-input>
    </el-form-item>
    <el-form-item class="login-animation-two">
      <el-input :type="state.isShowPassword ? 'text' : 'password'"
        :placeholder="$t('message.account.accountPlaceholder2')" v-model="state.loginForm.password" autocomplete="off">
        <template #prefix>
          <el-icon class="el-input__icon">
            <elementUnlock />
          </el-icon>
        </template>
        <template #suffix>
          <i class="iconfont el-input__icon login-content-password"
            :class="state.isShowPassword ? 'icon-yincangmima' : 'icon-xianshimima'"
            @click="state.isShowPassword = !state.isShowPassword">
          </i>
        </template>
      </el-input>
    </el-form-item>
    <el-form-item class="login-animation-three">
      <div class="login-captcha-wrapper">
        <el-input type="text" maxlength="6" placeholder="请输入图形验证码"
          v-model="state.loginForm.code" clearable autocomplete="off" style="flex: 1;">
          <template #prefix>
            <el-icon class="el-input__icon">
              <elementPicture />
            </el-icon>
          </template>
        </el-input>
        <el-button
          v-if="!state.base64Image"
          class="get-captcha-btn"
          round
          @click="getCaptcha"
          :loading="state.getCaptchaLoading">
          获取验证码
        </el-button>
        <img v-if="state.base64Image"
          :key="state.timestamp"
          :src="state.base64Image"
          alt="captcha"
          class="captcha-image"
          @click="getCaptcha"
          title="点击刷新验证码" />
      </div>
    </el-form-item>
    <el-form-item class="login-animation-four">
      <el-button type="primary" class="login-content-submit" round @click="openVerify" :loading="state.loading.signIn">
        <span>{{ $t("message.account.accountBtnText") }}</span>
      </el-button>
    </el-form-item>
  </el-form>

  <!-- TOTP 二因素验证步骤：登录 step1 返回 needMfa 后显示 -->
  <el-form v-else size="large" class="login-content-form">
    <el-form-item>
      <el-input type="text" maxlength="6" placeholder="输入动态验证码" v-model="state.mfaCode" clearable
        autocomplete="off" @keyup.enter="onMfaVerify">
        <template #prefix>
          <el-icon class="el-input__icon">
            <elementPosition />
          </el-icon>
        </template>
      </el-input>
    </el-form-item>
    <el-form-item>
      <el-button type="primary" class="login-content-submit" round @click="onMfaVerify" :loading="state.loading.signIn">
        <span>验证并登录</span>
      </el-button>
    </el-form-item>
    <el-form-item>
      <el-button text type="primary" @click="backToLogin">
        <span>重新登录</span>
      </el-button>
    </el-form-item>
  </el-form>

  <!-- <el-dialog v-model="state.dialogVerifyVisible" title="旋转验证码" width="300px" center>
    <DragVerifyImgRotate ref="dragRef" :imgsrc="state.imgThree" v-model:isPassing="state.isPassingFour" text="请按住滑块拖动"
      successText="验证通过" handlerIcon="iconfont icon-step" successIcon="fa fa-hand-peace-o" @passcallback="passVerify" />
  </el-dialog> -->
</template>

<script setup lang="ts">
import { onMounted, ref, reactive, computed, getCurrentInstance } from "vue";
import { useRoute, useRouter } from "vue-router";
import { ElMessage } from "element-plus";
import { useI18n } from "vue-i18n";
import { initBackEndControlRoutes } from "@/router/index";
import { Session } from "@/utils/storage";
import Cookies from 'js-cookie';
import { signIn, captcha as getCaptchaApi } from "@/api/login/index";
import { mfaVerify } from "@/api/mfa";
import { formatAxis } from "@/utils/formatTime";
import rotate from '@/assets/rotate.png'

// 旋转图片滑块组件
// import DragVerifyImgRotate from "@/components/dragVerify/dragVerifyImgRotate.vue";

const { t } = useI18n();
const { proxy } = getCurrentInstance() as any;
const loginFormRef: any = ref(null);
const dragRef: any = ref(null);

const route = useRoute();
const router = useRouter();
const state = reactive({
  dialogVerifyVisible: false,
  isPassingFour: false,
  imgThree: rotate,
  loginForm: {
    username: "admin",
    password: "123456",
    code: "",
    // 图形验证码会话 ID（KVA 后端 getCaptcha 返回 captchaId）
    captchaId: "",
  },
  // 图形验证码
  base64Image: "",
  timestamp: Date.now(),
  getCaptchaLoading: false,
  // TOTP 两段式登录：needMfa=true 时进入二因素步骤
  isMfaStep: false,
  mfaToken: "",
  mfaCode: "",
  rules: {
    username: [{ required: true, message: "请输入用户名", trigger: "blur" }],
    password: [{ required: true, message: "请输入密码", trigger: "blur" }],
  },
  isShowPassword: false,
  loading: {
    signIn: false,
  },
});
// 获取图形验证码（KVA: GET /system/user/getCaptcha → { base64Captcha, captchaId }）
const getCaptcha = async () => {
  state.getCaptchaLoading = true;
  try {
    const res: any = await getCaptchaApi();
    // request 成功时已解包 data；兼容返回 {data:{...}} 包装的形态
    const data = res?.data ?? res;
    const img = data?.base64Captcha || data?.base64Image;
    if (img) {
      state.base64Image = img;
      state.loginForm.captchaId = data?.captchaId || "";
      state.timestamp = Date.now();
      state.loginForm.code = "";
    }
  } catch (e) {
    ElMessage.error("获取验证码失败：" + (e as Error).message);
  } finally {
    state.getCaptchaLoading = false;
  }
};

onMounted(() => {
  getCaptcha();
});


// 时间获取
const currentTime = computed(() => {
  return formatAxis(new Date());
});
// 校验登录表单并登录
const login = () => {
  loginFormRef.value.validate((valid: boolean) => {
    if (valid) {
      onSignIn();
    } else {
      return false;
    }
  });
};
// 登录
const onSignIn = async () => {
  state.loading.signIn = true;
  let loginRespon;
  try {
    // 提交账号、密码、图形验证码及 captchaId
    loginRespon = await signIn(state.loginForm);
  } catch (e) {
    // dragRef.value.reset();
    // state.isPassingFour = false;
    state.loading.signIn = false;
    state.loginForm.code = "";
    getCaptcha();
    return;
  }
  // request 成功时已解包 data；兼容返回 {data:{...}} 包装的形态
  let loginRes = loginRespon?.data ?? loginRespon;
  // TOTP 两段式登录：needMfa=true → 进入二因素验证步骤（不签发正式 token）
  if (loginRes?.needMfa) {
    state.mfaToken = loginRes.mfaToken;
    state.isMfaStep = true;
    state.loading.signIn = false;
    return;
  }
  Session.set("token", loginRes.token);
  Cookies.set('userName', state.loginForm.username);
  // 模拟后端控制路由，isRequestRoutes 为 true，则开启后端控制路由
  // 添加完动态路由，再进行 router 跳转，否则可能报错 No match found for location with path "/"
  await initBackEndControlRoutes();
  // 执行完 initBackEndControlRoutes，再执行 signInSuccess
  signInSuccess();
};
const openVerify = () => {
  // state.dialogVerifyVisible = true;
  login();
};

// 退出二因素步骤，返回账号密码登录
const backToLogin = () => {
  state.isMfaStep = false;
  state.mfaToken = "";
  state.mfaCode = "";
};

// TOTP 二因素验证：动态码 → /mfa/verify 换正式 token → 登录成功
const onMfaVerify = async () => {
  if (!state.mfaCode) {
    ElMessage.warning("请输入动态验证码");
    return;
  }
  state.loading.signIn = true;
  try {
    const verifyRes: any = await mfaVerify(state.mfaToken, state.mfaCode);
    const vRes = verifyRes?.data ?? verifyRes;
    if (!vRes?.token) {
      ElMessage.error("验证失败，请重试");
      state.mfaCode = "";
      return;
    }
    Session.set("token", vRes.token);
    Cookies.set('userName', state.loginForm.username);
    await initBackEndControlRoutes();
    signInSuccess();
  } catch (e) {
    state.mfaCode = "";
  } finally {
    state.loading.signIn = false;
  }
};
const passVerify = () => {
  state.dialogVerifyVisible = false;
  state.isPassingFour = false;
  login();
};

// 登录成功后的跳转
const signInSuccess = () => {
  // 初始化登录成功时间问候语
  let currentTimeInfo = currentTime.value;
  // 登录成功，跳到转首页
  // 添加完动态路由，再进行 router 跳转，否则可能报错 No match found for location with path "/"
  // 如果是复制粘贴的路径，非首页/登录页，那么登录成功后重定向到对应的路径中
  console.log(route.query?.redirect, "route.query?.redirect")
  if (route.query?.redirect) {
    router.push({
      path: route.query?.redirect,
      query:
        Object.keys(route.query?.params).length > 0
          ? JSON.parse(route.query?.params)
          : "",
    });
  } else {
    router.push("/");
  }
  //登录成功提示
  setTimeout(() => {
    // 关闭 loading
    state.loading.signIn = true;
    const signInText = t("message.signInText");
    ElMessage.success(`${currentTimeInfo}，${signInText}`);
    // 修复防止退出登录再进入界面时，需要刷新样式才生效的问题，初始化布局样式等(登录的时候触发，目前方案)
    proxy.mittBus.emit("onSignInClick");
  }, 300);
};
</script>

<style scoped lang="scss">
.login-content-form {
  margin-top: 20px;

  .login-animation-one,
  .login-animation-two,
  .login-animation-three,
  .login-animation-four {
    opacity: 0;
    animation-name: error-num;
    animation-duration: 0.5s;
    animation-fill-mode: forwards;
  }

  .login-animation-one {
    animation-delay: 0.1s;
  }

  .login-animation-two {
    animation-delay: 0.2s;
  }

  .login-animation-three {
    animation-delay: 0.3s;
  }

  .login-animation-four {
    animation-delay: 0.4s;
    margin-bottom: 5px;
  }

  .login-captcha-wrapper {
    display: flex;
    align-items: center;
    gap: 10px;
    width: 100%;

    .el-input {
      flex: 1;
    }

    .get-captcha-btn {
      width: 110px;
      white-space: nowrap;
      height: 40px;
      flex-shrink: 0;

      &:disabled {
        opacity: 0.5;
        cursor: not-allowed;
      }
    }

    .captcha-image {
      width: 120px;
      height: 40px;
      object-fit: cover;
      border-radius: 4px;
      cursor: pointer;
      border: 1px solid #dcdfe6;
      flex-shrink: 0;

      &:hover {
        border-color: #c0c4cc;
        opacity: 0.9;
      }
    }
  }

  .login-content-password {
    display: inline-block;
    width: 25px;
    cursor: pointer;

    &:hover {
      color: #909399;
    }
  }

  .login-content-code {
    display: flex;
    align-items: center;
    justify-content: space-around;

    .login-content-code-img {
      width: 100%;
      height: 40px;
      line-height: 40px;
      background-color: #ffffff;
      border: 1px solid rgb(220, 223, 230);
      color: #333;
      font-size: 16px;
      font-weight: 700;
      letter-spacing: 5px;
      text-indent: 5px;
      text-align: center;
      cursor: pointer;
      transition: all ease 0.2s;
      border-radius: 4px;
      user-select: none;

      &:hover {
        border-color: #c0c4cc;
        transition: all ease 0.2s;
      }
    }
  }

  .login-content-submit {
    width: 100%;
    letter-spacing: 2px;
    font-weight: 300;
    margin-top: 15px;
  }
}
</style>
