<script setup>
import { computed, onBeforeUnmount, onMounted, provide, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { clearSession } from './session.js'

const route = useRoute()
const router = useRouter()
const theme = ref(localStorage.getItem('iblog-theme-mode') || 'auto')
const colorScheme = window.matchMedia('(prefers-color-scheme: dark)')
const systemPrefersDark = ref(colorScheme.matches)
const effectiveDark = computed(() =>
  theme.value === 'auto' ? systemPrefersDark.value : theme.value === 'dark',
)
function applyTheme() {
  document.documentElement.dataset.theme = effectiveDark.value ? 'dark' : 'light'
  document
    .querySelector('meta[name="theme-color"]')
    ?.setAttribute('content', effectiveDark.value ? '#000000' : '#f5f5f7')
  localStorage.setItem('iblog-theme-mode', theme.value)
}
function toggleTheme() {
  theme.value = effectiveDark.value ? 'light' : 'dark'
}
function syncSystemTheme(event) {
  systemPrefersDark.value = event.matches
}
watch([theme, systemPrefersDark], applyTheme, { immediate: true })
onMounted(() => colorScheme.addEventListener('change', syncSystemTheme))
onBeforeUnmount(() => colorScheme.removeEventListener('change', syncSystemTheme))
const notices = ref([])
let nextId = 0
function toast(kind, text) {
  const id = ++nextId
  notices.value.push({ id, kind, text })
  setTimeout(() => {
    notices.value = notices.value.filter((item) => item.id !== id)
  }, 3200)
}
const confirmDialog = ref(null)
function confirmAction(message, title = '请确认') {
  return new Promise((resolve) => {
    confirmDialog.value?.resolve(false)
    confirmDialog.value = { message, title, resolve }
    requestAnimationFrame(() => document.querySelector('.confirm-dialog button')?.focus())
  })
}
function finishConfirm(value) {
  const current = confirmDialog.value
  confirmDialog.value = null
  current?.resolve(value)
}
provide('toast', toast)
provide('confirmAction', confirmAction)
const nav = [
  ['/', '文章管理'],
  ['/links', '友链管理'],
  ['/tags', '标签管理'],
  ['/about', '关于管理'],
  ['/settings', '站点设置'],
  ['/account', '账号管理'],
]
async function logout() {
  try {
    await fetch('/api/v1/auth/logout', {
      method: 'POST',
      credentials: 'same-origin',
      headers: { Accept: 'application/json' },
    })
  } finally {
    clearSession()
    router.push('/login')
  }
}
</script>

<template>
  <template v-if="route.path !== '/login'">
    <header class="nav">
      <div class="nav-inner">
        <nav class="nav-tabs" aria-label="管理导航">
          <RouterLink
            v-for="[to, label] in nav"
            :key="to"
            class="nav-tab"
            :class="{ active: to === '/' ? route.path === '/' : route.path.startsWith(to) }"
            :to="to"
            >{{ label }}</RouterLink
          >
        </nav>
        <div class="nav-actions">
          <button
            class="theme-toggle"
            type="button"
            :aria-label="effectiveDark ? '切换到明亮模式' : '切换到暗色模式'"
            @click="toggleTheme"
          >
            <svg v-if="effectiveDark" viewBox="0 0 24 24" aria-hidden="true">
              <circle cx="12" cy="12" r="4" />
              <path
                d="M12 2v2m0 16v2M4.93 4.93l1.42 1.42m11.3 11.3 1.42 1.42M2 12h2m16 0h2M4.93 19.07l1.42-1.42m11.3-11.3 1.42-1.42"
              /></svg
            ><svg v-else viewBox="0 0 24 24" aria-hidden="true">
              <path d="M20.9 13A9 9 0 0 1 11 3.1 9 9 0 1 0 20.9 13Z" />
            </svg></button
          ><button class="link-btn" type="button" @click="logout">退出登录</button>
        </div>
      </div>
    </header>
  </template>
  <a class="skip-link" href="#admin-content">跳到主要内容</a>
  <div class="app-page"><RouterView /></div>
  <div class="toast-host" aria-live="polite" aria-atomic="false">
    <div v-for="item in notices" :key="item.id" class="toast" :class="item.kind" role="status">
      <span class="toast-icon" aria-hidden="true">{{
        item.kind === 'success' ? '✓' : item.kind === 'error' ? '!' : 'i'
      }}</span
      ><span class="toast-message">{{ item.text }}</span
      ><button
        class="toast-close"
        type="button"
        aria-label="关闭提示"
        @click="notices = notices.filter((x) => x.id !== item.id)"
      >
        ×
      </button>
    </div>
  </div>
  <div v-if="confirmDialog" class="confirm-backdrop" @keydown.esc="finishConfirm(false)">
    <section
      class="confirm-dialog"
      role="dialog"
      aria-modal="true"
      :aria-label="confirmDialog.title"
    >
      <h2>{{ confirmDialog.title }}</h2>
      <p>{{ confirmDialog.message }}</p>
      <div class="confirm-actions">
        <button class="btn btn-secondary" type="button" @click="finishConfirm(false)">取消</button
        ><button class="btn" type="button" @click="finishConfirm(true)">确定</button>
      </div>
    </section>
  </div>
</template>
