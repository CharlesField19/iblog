<script setup>
import { inject, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { adminRequest } from '../api.js'
import { saveSession } from '../session.js'
import { request } from '../../../shared/api.js'

const router = useRouter(); const route = useRoute(); const toast = inject('toast')
const username = ref(''); const password = ref(''); const error = ref(''); const loading = ref(false)
const setupRequired = ref(false); const checking = ref(true)
request('/api/v1/auth/setup-status').then((data) => { setupRequired.value = Boolean(data.setupRequired) }).catch((e) => { error.value = e.message }).finally(() => { checking.value = false })
async function submit() {
  if (loading.value) return
  if (!username.value.trim() || !password.value) { error.value = '请输入用户名和密码'; return }
  if (setupRequired.value && password.value.length < 8) { error.value = '密码至少需要 8 个字符'; return }
  loading.value = true; error.value = ''
  try {
    const response = setupRequired.value
      ? await request('/api/v1/auth/setup', { method: 'POST', body: { username: username.value.trim(), password: password.value } })
      : await adminRequest('/api/v1/auth/login', { method: 'POST', body: { username: username.value.trim(), password: password.value } })
    saveSession(response); toast('success', setupRequired.value ? '管理员账号已创建' : '登录成功')
    router.replace(typeof route.query.next === 'string' ? route.query.next : '/')
  } catch (e) { error.value = e.message }
  finally { loading.value = false }
}
</script>

<template>
  <main class="login-wrap"><section class="login-card">
    <div class="login-logo"><span class="logo-letter">C</span></div><h1>{{ setupRequired ? '初始化博客' : '博客管理' }}</h1>
    <p v-if="setupRequired" class="setup-copy">首次使用，请创建管理员账号。</p>
    <p v-if="checking" class="setup-copy">正在检查安装状态…</p>
    <form v-else @submit.prevent="submit">
      <div class="field"><label for="username">用户名</label><input id="username" v-model="username" autocomplete="username" placeholder="用户名" required></div>
      <div class="field"><label for="password">{{ setupRequired ? '设置密码' : '密码' }}</label><input id="password" v-model="password" type="password" :autocomplete="setupRequired ? 'new-password' : 'current-password'" :placeholder="setupRequired ? '至少 8 个字符' : '密码'" :minlength="setupRequired ? 8 : undefined" required></div>
      <p v-if="error" class="login-error" role="alert">{{ error }}</p>
      <button class="btn login-btn" :disabled="loading">{{ loading ? (setupRequired ? '创建中…' : '登录中…') : (setupRequired ? '创建管理员账号' : '登录') }}</button>
    </form>
  </section></main>
</template>
