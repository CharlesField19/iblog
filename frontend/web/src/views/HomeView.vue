<script setup>
import { onMounted, ref } from 'vue'
import { request } from '../../../shared/api.js'
import PostCard from '../components/PostCard.vue'

const posts = ref([])
const contacts = ref({})
const site = ref({ name: 'iBlog', titlePrefix: "Chenghao's", titleAccent: 'Blog', subtitle: 'Code · Life · Ideas' })
const loading = ref(true)
const error = ref('')
const social = [
  { key: 'github', label: 'GitHub', icon: '<path d="M9 19c-5 1.5-5-2.5-7-3m14 6v-3.87a3.37 3.37 0 0 0-.94-2.61c3.14-.35 6.44-1.54 6.44-7A5.44 5.44 0 0 0 20 4.77 5.07 5.07 0 0 0 19.91 1S18.73.65 16 2.48a13.38 13.38 0 0 0-7 0C6.27.65 5.09 1 5.09 1A5.07 5.07 0 0 0 5 4.77 5.44 5.44 0 0 0 3.5 8.55c0 5.42 3.3 6.61 6.44 7A3.37 3.37 0 0 0 9 18.13V22"/>' },
  { key: 'x', label: 'X', icon: '<path d="M18.244 2.25h3.308l-7.227 8.26 8.502 11.24H16.17l-5.214-6.817L4.99 21.75H1.68l7.73-8.835L1.254 2.25H8.08l4.713 6.231z"/>' },
  { key: 'telegram', label: 'Telegram', icon: '<line x1="22" y1="2" x2="11" y2="13"/><polygon points="22 2 15 22 11 13 2 9 22 2"/>' },
]
function safeExternalUrl(value) {
  try { const url = new URL(value); return ['http:', 'https:'].includes(url.protocol) ? url.href : '' }
  catch { return '' }
}
async function load() {
  loading.value = true; error.value = ''
  try {
    const [articles, settings] = await Promise.all([
      request('/api/v1/articles?page=1&pageSize=3'), request('/api/v1/settings'),
    ])
    posts.value = articles.items || []
    contacts.value = settings.contacts || {}
    site.value = { ...site.value, ...(settings.site || {}) }
    document.title = site.value.name || 'iBlog'
  } catch (e) { error.value = e.message }
  finally { loading.value = false }
}
onMounted(load)
</script>

<template>
  <section class="profile"><div class="profile-inner">
    <h1>{{ site.titlePrefix }} <span class="grad">{{ site.titleAccent }}</span></h1>
    <p class="hero-sub">{{ site.subtitle }}</p>
    <div class="social-icons">
      <a v-for="site in social" :key="site.key" v-show="safeExternalUrl(contacts[site.key])" :href="safeExternalUrl(contacts[site.key]) || undefined" :aria-label="site.label" :title="site.label" target="_blank" rel="noopener noreferrer"><svg :class="`social-icon-${site.key}`" viewBox="0 0 24 24" :fill="site.key === 'x' ? 'currentColor' : 'none'" aria-hidden="true" v-html="site.icon"></svg></a>
    </div>
  </div></section>
  <section class="section"><div class="container">
    <div class="section-head"><h2 class="section-title">最新文章</h2></div>
    <p v-if="loading" class="muted">正在加载文章…</p>
    <div v-else-if="error" role="alert"><p>{{ error }}</p><button class="btn" @click="load">重试</button></div>
    <div v-else-if="posts.length" class="post-list"><PostCard v-for="post in posts" :key="post.id" :post="post" /></div>
    <p v-else class="muted">还没有文章。</p>
  </div></section>
</template>
