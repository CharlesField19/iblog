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
  { key: 'github', label: 'GitHub', icon: '<path d="M9 19c-4.3 1.4-4.3-2.5-6-3m12 6v-3.9a3.4 3.4 0 0 0-.9-2.7c3-.3 6.2-1.5 6.2-6.7a5.2 5.2 0 0 0-1.4-3.6 4.8 4.8 0 0 0-.1-3.6S17.7 1.2 15 3a13.4 13.4 0 0 0-7 0C5.3 1.2 4.2 1.5 4.2 1.5a4.8 4.8 0 0 0-.1 3.6 5.2 5.2 0 0 0-1.4 3.6c0 5.2 3.2 6.4 6.2 6.7A3.4 3.4 0 0 0 8 18v4"/>' },
  { key: 'x', label: 'X', icon: '<path d="M4 4l16 16M20 4 4 20"/>' },
  { key: 'telegram', label: 'Telegram', icon: '<path d="m22 2-7 20-4-9-9-4 20-7ZM22 2 11 13"/>' },
]
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
      <a v-for="site in social" :key="site.key" v-show="contacts[site.key]" :href="contacts[site.key]" :aria-label="site.label" target="_blank" rel="noopener noreferrer"><svg viewBox="0 0 24 24" aria-hidden="true" v-html="site.icon"></svg></a>
    </div>
  </div></section>
  <section class="section"><div class="container">
    <div class="section-head"><h2 class="section-title">最新文章</h2><RouterLink to="/articles">查看全部 →</RouterLink></div>
    <p v-if="loading" class="muted">正在加载文章…</p>
    <div v-else-if="error" role="alert"><p>{{ error }}</p><button class="btn" @click="load">重试</button></div>
    <div v-else-if="posts.length" class="post-list"><PostCard v-for="post in posts" :key="post.id" :post="post" /></div>
    <p v-else class="muted">还没有文章。</p>
  </div></section>
</template>
