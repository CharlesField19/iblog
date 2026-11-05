<script setup>
import { computed, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { useRoute } from 'vue-router'
import { request } from '../../shared/api.js'

const route = useRoute()
const theme = ref(localStorage.getItem('iblog-theme-mode') || 'auto')
const colorScheme = window.matchMedia('(prefers-color-scheme: dark)')
const systemPrefersDark = ref(colorScheme.matches)
const effectiveDark = computed(() => theme.value === 'auto' ? systemPrefersDark.value : theme.value === 'dark')
const site = ref({ footerName: 'Chenghao', footerText: '用代码与热爱搭建' })
const links = [
  { to: '/', label: '首页' }, { to: '/articles', label: '文章' },
  { to: '/friends', label: '友链' }, { to: '/about', label: '关于' },
]
function applyTheme() {
  document.documentElement.dataset.theme = effectiveDark.value ? 'dark' : 'light'
  document.querySelector('meta[name="theme-color"]')?.setAttribute('content', effectiveDark.value ? '#000000' : '#f5f5f7')
  localStorage.setItem('iblog-theme-mode', theme.value)
}
function toggleTheme() { theme.value = effectiveDark.value ? 'light' : 'dark' }
function syncSystemTheme(event) { systemPrefersDark.value = event.matches }
watch([theme, systemPrefersDark], applyTheme, { immediate: true })
onMounted(async () => {
  colorScheme.addEventListener('change', syncSystemTheme)
  try {
    const settings = await request('/api/v1/settings')
    site.value = { ...site.value, ...(settings.site || {}) }
  } catch { /* Keep the built-in footer copy when settings cannot be loaded. */ }
})
onBeforeUnmount(() => colorScheme.removeEventListener('change', syncSystemTheme))
</script>

<template>
  <div>
    <header class="nav">
      <div class="nav-inner">
        <nav class="nav-links" aria-label="主导航">
          <RouterLink v-for="link in links" :key="link.to" :to="link.to" :class="{ active: route.path === link.to }" :aria-current="route.path === link.to ? 'page' : undefined">{{ link.label }}</RouterLink>
        </nav>
        <div class="nav-actions">
          <button class="theme-toggle" type="button" :aria-label="effectiveDark ? '切换到明亮模式' : '切换到暗色模式'" @click="toggleTheme"><svg v-if="effectiveDark" class="icon-sun" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><circle cx="12" cy="12" r="5"/><line x1="12" y1="1" x2="12" y2="3"/><line x1="12" y1="21" x2="12" y2="23"/><line x1="4.22" y1="4.22" x2="5.64" y2="5.64"/><line x1="18.36" y1="18.36" x2="19.78" y2="19.78"/><line x1="1" y1="12" x2="3" y2="12"/><line x1="21" y1="12" x2="23" y2="12"/><line x1="4.22" y1="19.78" x2="5.64" y2="18.36"/><line x1="18.36" y1="5.64" x2="19.78" y2="4.22"/></svg><svg v-else class="icon-moon" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><path d="M21 12.79A9 9 0 1 1 11.21 3 7 7 0 0 0 21 12.79z"/></svg></button>
        </div>
      </div>
    </header>
    <a class="skip-link" href="#main-content">跳到主要内容</a>
    <main id="main-content" tabindex="-1"><RouterView /></main>
    <footer class="footer"><div class="footer-inner">
      <nav class="footer-links" aria-label="页脚导航"><RouterLink v-for="link in links" :key="link.to" :to="link.to">{{ link.label }}</RouterLink></nav>
      <p>© {{ new Date().getFullYear() }} {{ site.footerName }} · {{ site.footerText }}</p>
    </div></footer>
  </div>
</template>
