<script setup>
import { computed, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { request } from '../../../shared/api.js'
import PostCard from '../components/PostCard.vue'

const route = useRoute()
const router = useRouter()
const posts = ref([])
const tags = ref([])
const selected = ref(String(route.query.tag || ''))
const search = ref(String(route.query.q || ''))
const loading = ref(true)
const error = ref('')
const page = ref(1)
const pageSize = 12
const total = ref(0)
let timer
let requestId = 0
let requestController
const hasMore = computed(() => posts.value.length < total.value)
async function load({ append = false } = {}) {
  const currentRequest = ++requestId
  requestController?.abort()
  requestController = new AbortController()
  loading.value = true
  error.value = ''
  const targetPage = append ? page.value + 1 : 1
  const q = new URLSearchParams({ page: String(targetPage), pageSize: String(pageSize) })
  if (selected.value) q.set('tag', selected.value)
  if (search.value.trim()) q.set('q', search.value.trim())
  try {
    const result = await request(`/api/v1/articles?${q}`, { signal: requestController.signal })
    if (currentRequest !== requestId) return
    posts.value = append ? [...posts.value, ...(result.items || [])] : result.items || []
    page.value = targetPage
    total.value = result.total || 0
  } catch (e) {
    if (currentRequest !== requestId) return
    error.value = e.message
    if (!append) posts.value = []
  } finally {
    if (currentRequest === requestId) loading.value = false
  }
}
function select(name) {
  selected.value = selected.value === name ? '' : name
  page.value = 1
  router.replace({
    query: {
      ...(selected.value ? { tag: selected.value } : {}),
      ...(search.value.trim() ? { q: search.value.trim() } : {}),
    },
  })
  load()
}
function clearFilters() {
  search.value = ''
  selected.value = ''
  page.value = 1
  router.replace({ query: {} })
  load()
}
watch(search, () => {
  clearTimeout(timer)
  timer = setTimeout(() => {
    page.value = 1
    router.replace({
      query: {
        ...(selected.value ? { tag: selected.value } : {}),
        ...(search.value.trim() ? { q: search.value.trim() } : {}),
      },
    })
    load()
  }, 300)
})
watch(
  () => [route.query.tag, route.query.q],
  ([tag, q]) => {
    const nextTag = String(tag || '')
    const nextQ = String(q || '')
    if (selected.value !== nextTag || search.value !== nextQ) {
      selected.value = nextTag
      search.value = nextQ
      page.value = 1
      load()
    }
  },
)
onMounted(async () => {
  load()
  try {
    tags.value = await request('/api/v1/tags')
  } catch {}
})
onBeforeUnmount(() => {
  clearTimeout(timer)
  requestController?.abort()
})
</script>

<template>
  <section class="page-head">
    <div class="container article-list-container">
      <h1>文章</h1>
      <p class="sub">这里是我的文字。偶尔更新，尽量真诚。</p>
    </div>
  </section>
  <section class="section">
    <div class="container article-list-container">
      <label class="search-box"
        ><span class="search-icon" aria-hidden="true">⌕</span
        ><input
          v-model="search"
          type="search"
          placeholder="搜索文章标题、标签或摘要…"
          aria-label="搜索文章"
          autocomplete="off"
        /><button
          v-if="search"
          class="search-clear"
          type="button"
          aria-label="清空搜索"
          @click="search = ''"
        >
          ×
        </button></label
      >
      <div v-if="tags.length" class="skill-list filter-tags">
        <button
          v-for="tag in tags"
          :key="tag.name"
          class="chip"
          :class="{ selected: selected === tag.name }"
          @click="select(tag.name)"
        >
          {{ tag.name }} <span class="tag-count">{{ tag.count }}</span>
        </button>
      </div>
      <p v-if="loading" class="muted">正在加载…</p>
      <div v-else-if="error" role="alert">
        <p>{{ error }}</p>
        <button class="btn" @click="load">重试</button>
      </div>
      <div v-else-if="posts.length" class="post-list">
        <PostCard v-for="post in posts" :key="post.id" :post="post" />
      </div>
      <div v-if="!loading && !error && !posts.length" class="empty-state">
        <p>{{ search || selected ? '没有找到符合条件的文章。' : '还没有文章。' }}</p>
        <button v-if="search || selected" class="btn btn-secondary" @click="clearFilters">
          清空筛选
        </button>
      </div>
      <div v-if="posts.length" class="load-more-row">
        <span class="muted">已显示 {{ posts.length }} / {{ total }} 篇</span
        ><button
          v-if="hasMore"
          class="btn btn-secondary"
          :disabled="loading"
          @click="load({ append: true })"
        >
          {{ loading ? '加载中…' : '加载更多' }}
        </button>
      </div>
    </div>
  </section>
</template>
