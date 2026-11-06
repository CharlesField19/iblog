<script setup>
import { computed, inject, onMounted, ref, watch } from 'vue'
import { RouterLink } from 'vue-router'
import { adminRequest } from '../api.js'

const toast = inject('toast')
const confirmAction = inject('confirmAction')
const items = ref([])
const total = ref(0)
const page = ref(1)
const status = ref('')
const tag = ref('')
const tags = ref([])
const search = ref('')
const query = ref('')
const sort = ref('updated')
const loading = ref(false)
const error = ref('')
const selected = ref([])
const pageSize = 10
const pages = computed(() => Math.max(1, Math.ceil(total.value / pageSize)))
let timer
async function load() {
  loading.value = true
  error.value = ''
  const p = new URLSearchParams({
    page: String(page.value),
    pageSize: String(pageSize),
    sort: sort.value,
  })
  if (status.value) p.set('status', status.value)
  if (tag.value) p.set('tag', tag.value)
  if (query.value) p.set('q', query.value)
  try {
    const d = await adminRequest(`/api/v1/admin/articles?${p}`)
    items.value = d.items || []
    total.value = d.total || 0
    selected.value = []
  } catch (e) {
    error.value = e.message
  } finally {
    loading.value = false
  }
}
watch([status, tag, page, sort], load)
watch(search, () => {
  clearTimeout(timer)
  timer = setTimeout(() => {
    query.value = search.value.trim()
    page.value = 1
    load()
  }, 250)
})
onMounted(load)
onMounted(async () => {
  try {
    tags.value = (await adminRequest('/api/v1/admin/tags')).items || []
  } catch {}
})
async function remove(post) {
  if (!(await confirmAction(`删除文章「${post.title}」？此操作不可撤销。`, '删除文章'))) return
  try {
    await adminRequest(`/api/v1/admin/articles/${post.id}`, { method: 'DELETE' })
    toast('success', '文章已删除')
    if (items.value.length === 1 && page.value > 1) page.value--
    else load()
  } catch (e) {
    toast('error', e.message)
  }
}
async function batch(action) {
  if (!selected.value.length) return
  if (
    action === 'delete' &&
    !(await confirmAction(
      `确定删除选中的 ${selected.value.length} 篇文章？此操作不可撤销。`,
      '批量删除',
    ))
  )
    return
  const removePage =
    action === 'delete' && selected.value.length === items.value.length && page.value > 1
  try {
    await adminRequest('/api/v1/admin/articles/batch', {
      method: 'POST',
      body: { ids: selected.value, action },
    })
    toast(
      'success',
      action === 'publish'
        ? '已批量发布'
        : action === 'archive'
          ? '已批量归档（转为草稿）'
          : '已批量删除',
    )
    if (removePage) page.value--
    else load()
  } catch (e) {
    toast('error', e.message)
  }
}
function setStatus(value) {
  status.value = value
  page.value = 1
}
function formatDate(s) {
  return s
    ? new Intl.DateTimeFormat('zh-CN', {
        year: 'numeric',
        month: 'short',
        day: 'numeric',
        hour: '2-digit',
        minute: '2-digit',
        hour12: false,
      }).format(new Date(s))
    : '—'
}
</script>

<template>
  <main id="admin-content" class="admin-main" tabindex="-1">
    <div class="list-head">
      <h1>
        文章管理 <span class="count">{{ total }}</span>
      </h1>
      <RouterLink class="btn" to="/new">新建文章</RouterLink>
    </div>
    <div class="toolbar">
      <div class="pills" role="group" aria-label="按文章状态筛选">
        <button
          v-for="item in [
            { value: '', label: '全部' },
            { value: 'draft', label: '草稿' },
            { value: 'published', label: '已发布' },
          ]"
          :key="item.value"
          class="pill"
          :class="{ active: status === item.value }"
          :aria-pressed="status === item.value"
          @click="setStatus(item.value)"
        >
          {{ item.label }}
        </button>
      </div>
      <label class="sort-control"
        >标签<select v-model="tag" @change="page = 1">
          <option value="">全部标签</option>
          <option v-for="item in tags" :key="item.id" :value="item.name">
            {{ item.name }}（{{ item.count }}）
          </option>
        </select></label
      ><label class="search-box"
        ><span aria-hidden="true">⌕</span
        ><input
          v-model="search"
          type="search"
          placeholder="搜索标题、标签或摘要…"
          aria-label="搜索文章"
          autocomplete="off" /></label
      ><label class="sort-control"
        >排序<select v-model="sort">
          <option value="updated">最近更新</option>
          <option value="title">标题</option>
          <option value="views">浏览量</option>
        </select></label
      >
    </div>
    <p v-if="error" class="card error-card" role="alert">
      {{ error }} <button class="link-btn" @click="load">重试</button>
    </p>
    <div v-else-if="loading" class="stack">
      <div v-for="n in 3" :key="n" class="card skeleton sk-line"></div>
    </div>
    <div v-else-if="!items.length" class="card empty-state">
      {{ query || status ? '没有符合条件的文章。' : '还没有文章，点击“新建文章”开始写作。' }}
    </div>
    <div v-else class="stack">
      <article v-for="post in items" :key="post.id" class="card admin-row">
        <label class="row-check"
          ><input
            v-model="selected"
            type="checkbox"
            :value="post.id"
            :aria-label="`选择文章：${post.title}`"
        /></label>
        <div class="row-main">
          <div class="row-title">
            <h3>{{ post.title }}</h3>
            <span class="status-pill" :class="post.status">{{
              post.status === 'published' ? '已发布' : '草稿'
            }}</span>
          </div>
          <p class="row-meta">
            <span class="slug">{{ post.slug }}</span
            ><time :datetime="post.updatedAt">{{ formatDate(post.updatedAt) }}</time
            ><span>{{ new Intl.NumberFormat('zh-CN').format(post.views) }} 次浏览</span>
          </p>
          <div class="tag-list">
            <span v-for="tag in post.tags" :key="tag" class="tag">{{ tag }}</span>
          </div>
        </div>
        <div class="row-actions">
          <RouterLink class="link-btn" :to="`/edit/${post.id}`">编辑</RouterLink
          ><button class="link-btn danger" @click="remove(post)">删除</button>
        </div>
      </article>
    </div>
    <div v-if="!loading && items.length" class="batch-bar">
      <label
        ><input
          type="checkbox"
          :checked="items.length > 0 && selected.length === items.length"
          @change="selected = $event.target.checked ? items.map((item) => item.id) : []"
        />
        全选本页</label
      ><span>已选 {{ selected.length }} 篇</span
      ><button class="link-btn" :disabled="!selected.length" @click="batch('publish')">
        批量发布</button
      ><button
        class="link-btn"
        :disabled="!selected.length"
        title="取消公开展示，转为草稿状态"
        @click="batch('archive')"
      >
        批量归档</button
      ><button class="link-btn danger" :disabled="!selected.length" @click="batch('delete')">
        批量删除
      </button>
    </div>
    <div v-if="!loading && total" class="pager">
      <button class="btn btn-secondary btn-sm" :disabled="page <= 1" @click="page--">上一页</button
      ><span class="pager-info">第 {{ page }} / {{ pages }} 页 · 共 {{ total }} 篇</span
      ><button class="btn btn-secondary btn-sm" :disabled="page >= pages" @click="page++">
        下一页
      </button>
    </div>
  </main>
</template>
