<script setup>
import { errorMessage } from '../../../shared/i18n.js'
import { computed, inject, onMounted, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { RouterLink } from 'vue-router'
import { adminRequest } from '../api.js'

const toast = inject('toast')
const { t, locale } = useI18n()
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
    error.value = errorMessage(e)
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
  if (
    !(await confirmAction(
      t('admin.dashboard.deleteConfirm', { title: post.title }),
      t('admin.dashboard.delete'),
    ))
  )
    return
  try {
    await adminRequest(`/api/v1/admin/articles/${post.id}`, { method: 'DELETE' })
    toast('success', t('admin.dashboard.deleted'))
    if (items.value.length === 1 && page.value > 1) page.value--
    else load()
  } catch (e) {
    toast('error', errorMessage(e))
  }
}
async function batch(action) {
  if (!selected.value.length) return
  if (
    action === 'delete' &&
    !(await confirmAction(
      t('admin.dashboard.deleteSelected', { count: selected.value.length }),
      t('admin.dashboard.batchDelete'),
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
        ? t('admin.dashboard.publishConfirm')
        : action === 'archive'
          ? t('admin.dashboard.archived')
          : t('admin.dashboard.deletedMany'),
    )
    if (removePage) page.value--
    else load()
  } catch (e) {
    toast('error', errorMessage(e))
  }
}
function setStatus(value) {
  status.value = value
  page.value = 1
}
function formatDate(s) {
  return s
    ? new Intl.DateTimeFormat(locale.value, {
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
        {{ t('admin.dashboard.title') }} <span class="count">{{ total }}</span>
      </h1>
      <RouterLink class="btn" to="/new">{{ t('admin.dashboard.newArticle') }}</RouterLink>
    </div>
    <div class="toolbar">
      <div class="pills" role="group" :aria-label="t('admin.dashboard.statusFilter')">
        <button
          v-for="item in [
            { value: '', label: t('admin.dashboard.all') },
            { value: 'draft', label: t('admin.dashboard.draft') },
            { value: 'published', label: t('admin.dashboard.published') },
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
        >{{ t('admin.dashboard.tags')
        }}<select v-model="tag" @change="page = 1">
          <option value="">{{ t('admin.dashboard.allTags') }}</option>
          <option v-for="item in tags" :key="item.id" :value="item.name">
            {{ item.name }}（{{ item.count }}）
          </option>
        </select></label
      ><label class="search-box"
        ><span aria-hidden="true">⌕</span
        ><input
          v-model="search"
          type="search"
          :placeholder="t('admin.dashboard.search')"
          :aria-label="t('admin.dashboard.searchLabel')"
          autocomplete="off" /></label
      ><label class="sort-control"
        >{{ t('admin.dashboard.sort')
        }}<select v-model="sort">
          <option value="updated">{{ t('admin.dashboard.updated') }}</option>
          <option value="title">{{ t('admin.dashboard.titleSort') }}</option>
          <option value="views">{{ t('admin.dashboard.views') }}</option>
        </select></label
      >
    </div>
    <p v-if="error" class="card error-card" role="alert">
      {{ error }} <button class="link-btn" @click="load">{{ t('admin.dashboard.retry') }}</button>
    </p>
    <div v-else-if="loading" class="stack">
      <div v-for="n in 3" :key="n" class="card skeleton sk-line"></div>
    </div>
    <div v-else-if="!items.length" class="card empty-state">
      {{ query || status ? t('admin.dashboard.noMatches') : t('admin.dashboard.empty') }}
    </div>
    <div v-else class="stack">
      <article v-for="post in items" :key="post.id" class="card admin-row">
        <label class="row-check"
          ><input
            v-model="selected"
            type="checkbox"
            :value="post.id"
            :aria-label="t('admin.dashboard.selectArticle', { title: post.title })"
        /></label>
        <div class="row-main">
          <div class="row-title">
            <h3>{{ post.title }}</h3>
            <span class="status-pill" :class="post.status">{{
              post.status === 'published'
                ? t('admin.dashboard.statusPublished')
                : t('admin.dashboard.draft')
            }}</span>
          </div>
          <p class="row-meta">
            <span class="slug">{{ post.slug }}</span
            ><time :datetime="post.updatedAt">{{ formatDate(post.updatedAt) }}</time
            ><span>{{
              t('admin.dashboard.viewCount', {
                count: new Intl.NumberFormat(locale.value).format(post.views),
              })
            }}</span>
          </p>
          <div class="tag-list">
            <span v-for="tag in post.tags" :key="tag" class="tag">{{ tag }}</span>
          </div>
        </div>
        <div class="row-actions">
          <RouterLink class="link-btn" :to="`/edit/${post.id}`">{{
            t('admin.dashboard.edit')
          }}</RouterLink
          ><button class="link-btn danger" @click="remove(post)">
            {{ t('admin.dashboard.delete') }}
          </button>
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
        {{ t('admin.dashboard.selectPage') }}</label
      ><span>{{ t('admin.dashboard.selected', { count: selected.length }) }}</span
      ><button class="link-btn" :disabled="!selected.length" @click="batch('publish')">
        {{ t('admin.dashboard.batchPublish') }}</button
      ><button
        class="link-btn"
        :disabled="!selected.length"
        :title="t('admin.dashboard.archiveTitle')"
        @click="batch('archive')"
      >
        {{ t('admin.dashboard.batchArchive') }}</button
      ><button class="link-btn danger" :disabled="!selected.length" @click="batch('delete')">
        {{ t('admin.dashboard.batchDelete') }}
      </button>
    </div>
    <div v-if="!loading && total" class="pager">
      <button class="btn btn-secondary btn-sm" :disabled="page <= 1" @click="page--">
        {{ t('admin.dashboard.prev') }}</button
      ><span class="pager-info">{{ t('admin.dashboard.pager', { page, pages, total }) }}</span
      ><button class="btn btn-secondary btn-sm" :disabled="page >= pages" @click="page++">
        {{ t('admin.dashboard.next') }}
      </button>
    </div>
  </main>
</template>
