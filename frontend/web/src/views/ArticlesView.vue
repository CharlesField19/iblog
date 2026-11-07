<script setup lang="ts">
import { errorMessage } from '../../../shared/i18n'
import { computed, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { useRoute, useRouter } from 'vue-router'
import { getPublicData } from '../../../shared/publicData'
import type { ArticleSummary, PublicTag, PageResult } from '../../../shared/contracts'
import PostCard from '../components/PostCard.vue'
import PostCardSkeleton from '../components/PostCardSkeleton.vue'

const route = useRoute()
const router = useRouter()
const { t } = useI18n()
const posts = ref<ArticleSummary[]>([])
const tags = ref<PublicTag[]>([])
const selected = ref(String(route.query.tag || ''))
const search = ref(String(route.query.q || ''))
const loading = ref(true)
const error = ref('')
const page = ref(1)
const pageSize = 12
const total = ref(0)
let timer: ReturnType<typeof setTimeout> | undefined
let requestId = 0
const hasMore = computed(() => posts.value.length < total.value)
async function load({ append = false } = {}) {
  const currentRequest = ++requestId
  loading.value = true
  error.value = ''
  const targetPage = append ? page.value + 1 : 1
  const q = new URLSearchParams({ page: String(targetPage), pageSize: String(pageSize) })
  if (selected.value) q.set('tag', selected.value)
  if (search.value.trim()) q.set('q', search.value.trim())
  try {
    const result = await getPublicData<PageResult<ArticleSummary>>(`/api/v1/articles?${q}`)
    if (currentRequest !== requestId) return
    posts.value = append ? [...posts.value, ...(result.items || [])] : result.items || []
    page.value = targetPage
    total.value = result.total || 0
  } catch (e) {
    if (currentRequest !== requestId) return
    error.value = errorMessage(e)
    if (!append) posts.value = []
  } finally {
    if (currentRequest === requestId) loading.value = false
  }
}
function select(name: string) {
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
    tags.value = await getPublicData<PublicTag[]>('/api/v1/tags')
  } catch {}
})
onBeforeUnmount(() => {
  clearTimeout(timer)
})
</script>

<template>
  <section class="page-head">
    <div class="page-container article-list-container">
      <h1>{{ t('web.articles') }}</h1>
      <p class="sub">{{ t('web.articlesIntro') }}</p>
    </div>
  </section>
  <section class="section">
    <div class="page-container article-list-container">
      <label class="search-box"
        ><span class="search-icon" aria-hidden="true">⌕</span
        ><input
          v-model="search"
          type="search"
          :placeholder="t('web.searchPlaceholder')"
          :aria-label="t('web.searchLabel')"
          autocomplete="off"
        /><button
          v-if="search"
          class="search-clear"
          type="button"
          :aria-label="t('web.clearSearch')"
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
      <div v-if="loading" class="post-list"><PostCardSkeleton v-for="i in 4" :key="i" /></div>
      <div v-else-if="error" role="alert">
        <p>{{ error }}</p>
        <button class="btn" @click="load()">{{ t('web.retry') }}</button>
      </div>
      <div v-else-if="posts.length" class="post-list">
        <PostCard v-for="post in posts" :key="post.id" :post="post" />
      </div>
      <div v-if="!loading && !error && !posts.length" class="empty-state">
        <p>{{ search || selected ? t('web.noMatches') : t('web.noArticlesYet') }}</p>
        <button v-if="search || selected" class="btn btn-secondary" @click="clearFilters">
          {{ t('web.clearFilters') }}
        </button>
      </div>
      <div v-if="posts.length" class="load-more-row">
        <span class="muted">{{ t('web.shown', { shown: posts.length, total }) }}</span
        ><button
          v-if="hasMore"
          class="btn btn-secondary"
          :disabled="loading"
          @click="load({ append: true })"
        >
          {{ loading ? t('web.loadingMore') : t('web.loadMore') }}
        </button>
      </div>
    </div>
  </section>
</template>
