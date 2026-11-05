<script setup>
import { provide, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { clearSession } from './session.js'

const route = useRoute(); const router = useRouter()
const theme = ref(localStorage.getItem('admin-theme') || 'light')
function applyTheme() {
  document.documentElement.dataset.theme = theme.value
  localStorage.setItem('admin-theme', theme.value)
}
function toggleTheme() { theme.value = theme.value === 'dark' ? 'light' : 'dark' }
watch(theme, applyTheme, { immediate: true })
const notices = ref([]); let nextId = 0
function toast(kind, text) {
  const id = ++nextId; notices.value.push({ id, kind, text })
  setTimeout(() => { notices.value = notices.value.filter((item) => item.id !== id) }, 3200)
}
provide('toast', toast)
const nav = [
  ['/', '文章管理'], ['/links', '友链管理'], ['/tags', '标签管理'],
  ['/about', '关于管理'], ['/settings', '站点设置'], ['/account', '账号管理'],
]
function logout() { clearSession(); router.push('/login') }
</script>

<template>
  <template v-if="route.path !== '/login'">
    <header class="nav"><div class="nav-inner">
      <RouterLink class="brand" to="/"><span class="brand-text">博客管理</span></RouterLink>
      <nav class="nav-tabs" aria-label="管理导航"><RouterLink v-for="[to, label] in nav" :key="to" class="nav-tab" :class="{ active: to === '/' ? route.path === '/' : route.path.startsWith(to) }" :to="to">{{ label }}</RouterLink></nav>
      <div class="nav-actions"><button class="theme-toggle" type="button" :aria-label="theme === 'dark' ? '切换到明亮模式' : '切换到暗色模式'" @click="toggleTheme"><svg v-if="theme === 'dark'" viewBox="0 0 24 24" aria-hidden="true"><circle cx="12" cy="12" r="4"/><path d="M12 2v2m0 16v2M4.93 4.93l1.42 1.42m11.3 11.3 1.42 1.42M2 12h2m16 0h2M4.93 19.07l1.42-1.42m11.3-11.3 1.42-1.42"/></svg><svg v-else viewBox="0 0 24 24" aria-hidden="true"><path d="M20.9 13A9 9 0 0 1 11 3.1 9 9 0 1 0 20.9 13Z"/></svg></button><button class="link-btn" type="button" @click="logout">退出登录</button></div>
    </div></header>
  </template>
  <div class="app-page"><RouterView /></div>
  <div class="toast-host" aria-live="polite"><div v-for="item in notices" :key="item.id" class="toast" :class="item.kind" role="status">{{ item.text }}</div></div>
</template>
