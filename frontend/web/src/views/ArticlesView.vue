<script setup>
import { onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { request } from '../../../shared/api.js'
import PostCard from '../components/PostCard.vue'

const route = useRoute(); const router = useRouter()
const posts = ref([]); const tags = ref([]); const selected = ref(String(route.query.tag || ''))
const search = ref(''); const loading = ref(true); const error = ref(''); let timer
async function load() {
  loading.value = true; error.value = ''
  const q = new URLSearchParams({ page: '1', pageSize: '100' })
  if (selected.value) q.set('tag', selected.value)
  if (search.value.trim()) q.set('q', search.value.trim())
  try { posts.value = (await request(`/api/v1/articles?${q}`)).items || [] }
  catch (e) { error.value = e.message; posts.value = [] }
  finally { loading.value = false }
}
function select(name) {
  selected.value = selected.value === name ? '' : name
  router.replace({ query: selected.value ? { tag: selected.value } : {} })
  load()
}
watch(search, () => { clearTimeout(timer); timer = setTimeout(load, 300) })
watch(() => route.query.tag, (tag) => { if (selected.value !== (tag || '')) { selected.value = String(tag || ''); load() } })
onMounted(async () => { load(); try { tags.value = await request('/api/v1/tags') } catch {} })
onBeforeUnmount(() => clearTimeout(timer))
</script>

<template>
  <section class="page-head"><div class="container"><h1>文章</h1><p class="sub">这里是我的文字。偶尔更新，尽量真诚。</p></div></section>
  <section class="section"><div class="container">
    <label class="search-box"><span aria-hidden="true">⌕</span><input v-model="search" type="search" placeholder="搜索文章标题、标签或摘要..." aria-label="搜索文章"></label>
    <div v-if="tags.length" class="skill-list filter-tags"><button v-for="tag in tags" :key="tag.name" class="chip" :class="{ selected: selected === tag.name }" @click="select(tag.name)">{{ tag.name }} <span class="tag-count">{{ tag.count }}</span></button></div>
    <p v-if="loading" class="muted">正在加载…</p>
    <div v-else-if="error" role="alert"><p>{{ error }}</p><button class="btn" @click="load">重试</button></div>
    <div v-else-if="posts.length" class="post-list"><PostCard v-for="post in posts" :key="post.id" :post="post" /></div>
    <p v-else class="muted">没有找到文章。</p>
  </div></section>
</template>
