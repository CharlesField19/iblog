<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { useRoute } from 'vue-router'
import { i18n, toggleLocale } from '../../shared/i18n'
import { getPublicData } from '../../shared/publicData'
import type { PublicSettings } from '../../shared/contracts'

const route = useRoute()
const theme = ref(localStorage.getItem('iblog-theme-mode') || 'auto')
const colorScheme = window.matchMedia('(prefers-color-scheme: dark)')
const systemPrefersDark = ref(colorScheme.matches)
const effectiveDark = computed(() =>
  theme.value === 'auto' ? systemPrefersDark.value : theme.value === 'dark',
)
const site = ref({ footerName: 'Chenghao', footerText: '用代码与热爱搭建' })
const { t } = useI18n()
const links = computed(() => [
  { to: '/', label: t('web.home') },
  { to: '/articles', label: t('web.articles') },
  { to: '/friends', label: t('web.friends') },
  { to: '/about', label: t('web.about') },
])
function prefetchRoute(path: string) {
  if (path === '/articles') {
    void getPublicData('/api/v1/articles?page=1&pageSize=12').catch(() => {})
    void getPublicData('/api/v1/tags').catch(() => {})
  } else if (path === '/friends') {
    void getPublicData('/api/v1/friends').catch(() => {})
  } else if (path === '/about') {
    void getPublicData('/api/v1/settings').catch(() => {})
  }
}
const pageMeta = computed<Record<string, [string, string]>>(() => ({
  '/': ['iBlog · Code · Life · Ideas', t('web.articlesIntro')],
  '/articles': [`${t('web.articles')} · iBlog`, t('web.articlesIntro')],
  '/about': [`${t('web.about')} · iBlog`, t('web.aboutBlog')],
  '/friends': [`${t('web.friends')} · iBlog`, t('web.friendsIntro')],
}))
function applyRouteMeta(path = route.path) {
  if (path.startsWith('/articles/')) return
  const [title, description] = pageMeta.value[path] || pageMeta.value['/']
  document.title = title
  let meta = document.head.querySelector<HTMLMetaElement>('meta[name="description"]')
  if (!meta) {
    meta = document.createElement('meta')
    meta.name = 'description'
    document.head.appendChild(meta)
  }
  meta.content = description
  let canonical = document.head.querySelector<HTMLLinkElement>('link[rel="canonical"]')
  if (!canonical) {
    canonical = document.createElement('link')
    canonical.rel = 'canonical'
    document.head.appendChild(canonical)
  }
  canonical.href = new URL(path, window.location.origin).href
  const set = (key: string, value: string, property = false) => {
    const selector = property ? `meta[property="${key}"]` : `meta[name="${key}"]`
    let node = document.head.querySelector<HTMLMetaElement>(selector)
    if (!node) {
      node = document.createElement('meta')
      node.setAttribute(property ? 'property' : 'name', key)
      document.head.appendChild(node)
    }
    node.content = value
  }
  set('og:title', title, true)
  set('og:description', description, true)
  set('og:url', canonical.href, true)
  set('og:type', 'website', true)
  set('og:image', new URL('/favicon.ico', window.location.origin).href, true)
  set('twitter:card', 'summary')
  set('twitter:title', title)
  set('twitter:description', description)
  set('twitter:image', new URL('/favicon.ico', window.location.origin).href)
}
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
function syncSystemTheme(event: MediaQueryListEvent) {
  systemPrefersDark.value = event.matches
}
watch([theme, systemPrefersDark], applyTheme, { immediate: true })
watch([() => route.path, () => i18n.global.locale.value], () => applyRouteMeta(), {
  immediate: true,
})
onMounted(async () => {
  colorScheme.addEventListener('change', syncSystemTheme)
  try {
    const settings = await getPublicData<PublicSettings>('/api/v1/settings')
    site.value = { ...site.value, ...(settings.site || {}) }
  } catch {
    /* Keep the built-in footer copy when settings cannot be loaded. */
  }
})
onBeforeUnmount(() => colorScheme.removeEventListener('change', syncSystemTheme))
</script>

<template>
  <div>
    <header class="nav">
      <div class="nav-inner">
        <nav class="nav-links" :aria-label="$t('web.nav')">
          <RouterLink
            v-for="link in links"
            :key="link.to"
            :to="link.to"
            @pointerenter="prefetchRoute(link.to)"
            @focus="prefetchRoute(link.to)"
            @touchstart.passive="prefetchRoute(link.to)"
            :class="{ active: route.path === link.to }"
            :aria-current="route.path === link.to ? 'page' : undefined"
            >{{ link.label }}</RouterLink
          >
        </nav>
        <div class="nav-actions">
          <button
            class="language-toggle"
            type="button"
            :aria-label="$t('languageSwitch')"
            :title="$t('languageSwitch')"
            @click="toggleLocale"
          >
            <svg viewBox="0 0 24 24" aria-hidden="true">
              <path
                d="M3 5h12M9 3v2m3 0c-1 5-4 8-8 10m1-7c1.5 3 4 5.5 7 7m1 5 4-10 4 10m-6.5-3h5"
              />
            </svg>
          </button>
          <button
            class="theme-toggle"
            type="button"
            :aria-label="effectiveDark ? $t('themeLight') : $t('themeDark')"
            @click="toggleTheme"
          >
            <svg
              v-if="effectiveDark"
              class="icon-sun"
              viewBox="0 0 24 24"
              fill="none"
              stroke="currentColor"
              stroke-width="2"
              stroke-linecap="round"
              stroke-linejoin="round"
              aria-hidden="true"
            >
              <circle cx="12" cy="12" r="5" />
              <line x1="12" y1="1" x2="12" y2="3" />
              <line x1="12" y1="21" x2="12" y2="23" />
              <line x1="4.22" y1="4.22" x2="5.64" y2="5.64" />
              <line x1="18.36" y1="18.36" x2="19.78" y2="19.78" />
              <line x1="1" y1="12" x2="3" y2="12" />
              <line x1="21" y1="12" x2="23" y2="12" />
              <line x1="4.22" y1="19.78" x2="5.64" y2="18.36" />
              <line x1="18.36" y1="5.64" x2="19.78" y2="4.22" /></svg
            ><svg
              v-else
              class="icon-moon"
              viewBox="0 0 24 24"
              fill="none"
              stroke="currentColor"
              stroke-width="2"
              stroke-linecap="round"
              stroke-linejoin="round"
              aria-hidden="true"
            >
              <path d="M21 12.79A9 9 0 1 1 11.21 3 7 7 0 0 0 21 12.79z" />
            </svg>
          </button>
        </div>
      </div>
    </header>
    <a class="skip-link" href="#main-content">{{ $t('skipToContent') }}</a>
    <main id="main-content" tabindex="-1"><RouterView /></main>
    <footer class="footer">
      <div class="footer-inner">
        <nav class="footer-links" :aria-label="$t('web.footerNav')">
          <RouterLink v-for="link in links" :key="link.to" :to="link.to">{{
            link.label
          }}</RouterLink>
        </nav>
        <p>© {{ new Date().getFullYear() }} {{ site.footerName }} · {{ site.footerText }}</p>
      </div>
    </footer>
  </div>
</template>
