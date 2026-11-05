<script setup>
import { onMounted, ref, watch } from 'vue'
import { useRoute } from 'vue-router'
import { request } from '../../shared/api.js'

const route = useRoute()
const menuOpen = ref(false)
const theme = ref(localStorage.getItem('theme') || 'auto')
const site = ref({ footerName: 'Chenghao', footerText: '用代码与热爱搭建' })
const links = [
  { to: '/', label: '首页' }, { to: '/articles', label: '文章' },
  { to: '/friends', label: '友链' }, { to: '/about', label: '关于' },
]
function applyTheme() {
  const dark = theme.value === 'dark' || (theme.value === 'auto' && matchMedia('(prefers-color-scheme: dark)').matches)
  document.documentElement.dataset.theme = dark ? 'dark' : 'light'
  localStorage.setItem('theme', theme.value)
}
function toggleTheme() { theme.value = theme.value === 'dark' ? 'light' : 'dark' }
watch(theme, applyTheme, { immediate: true })
watch(() => route.fullPath, () => { menuOpen.value = false })
onMounted(async () => {
  try {
    const settings = await request('/api/v1/settings')
    site.value = { ...site.value, ...(settings.site || {}) }
  } catch { /* Keep the built-in footer copy when settings cannot be loaded. */ }
})
</script>

<template>
  <div :class="{ 'nav-open': menuOpen }">
    <header class="nav">
      <div class="nav-inner">
        <nav class="nav-links" aria-label="主导航">
          <RouterLink v-for="link in links" :key="link.to" :to="link.to" :class="{ active: route.path === link.to }">{{ link.label }}</RouterLink>
        </nav>
        <div class="nav-actions">
          <button class="theme-toggle" type="button" :aria-label="'切换主题，当前：' + theme" @click="toggleTheme"><svg v-if="theme !== 'dark'" viewBox="0 0 24 24" aria-hidden="true"><path d="M20.9 13A9 9 0 0 1 11 3.1 9 9 0 1 0 20.9 13Z"/></svg><svg v-else viewBox="0 0 24 24" aria-hidden="true"><circle cx="12" cy="12" r="4"/><path d="M12 2v2m0 16v2M4.93 4.93l1.42 1.42m11.3 11.3 1.42 1.42M2 12h2m16 0h2M4.93 19.07l1.42-1.42m11.3-11.3 1.42-1.42"/></svg></button>
          <button class="nav-toggle" type="button" :aria-expanded="menuOpen" aria-label="切换菜单" @click="menuOpen = !menuOpen"><span></span><span></span><span></span></button>
        </div>
      </div>
    </header>
    <main><RouterView /></main>
    <footer class="footer"><div class="footer-inner">
      <nav class="footer-links" aria-label="页脚导航"><RouterLink v-for="link in links" :key="link.to" :to="link.to">{{ link.label }}</RouterLink></nav>
      <p>© {{ new Date().getFullYear() }} {{ site.footerName }} · {{ site.footerText }}</p>
    </div></footer>
  </div>
</template>
