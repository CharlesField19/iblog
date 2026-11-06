<script setup>
import { errorMessage } from '../../../shared/i18n.js'
import { computed, inject, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { useRoute, useRouter } from 'vue-router'
import { onBeforeRouteLeave } from 'vue-router'
import { marked } from 'marked'
import DOMPurify from 'dompurify'
import { adminRequest } from '../api.js'

const route = useRoute()
const router = useRouter()
const toast = inject('toast')
const confirmAction = inject('confirmAction')
const { t } = useI18n()
const id = route.params.id
const title = ref('')
const slug = ref('')
const summary = ref('')
const content = ref('')
const status = ref('draft')
const tags = ref([])
const tagInput = ref('')
const knownTags = ref([])
const loading = ref(Boolean(id))
const saving = ref(false)
const error = ref('')
const initialSlug = ref('')
const initialStatus = ref('draft')
const lastSavedAt = ref('')
const previewHtml = ref('')
let previewTimer
let draftTimer
const draftKey = `iblog-editor-draft-${id || 'new'}`
const snapshot = () =>
  JSON.stringify({
    title: title.value,
    slug: slug.value,
    summary: summary.value,
    content: content.value,
    status: status.value,
    tags: tags.value,
  })
const baseline = ref('')
const dirty = computed(() => Boolean(baseline.value && snapshot() !== baseline.value))
const suggestions = computed(() =>
  knownTags.value
    .filter(
      (x) =>
        !tags.value.some((t) => t.toLowerCase() === x.toLowerCase()) &&
        (!tagInput.value || x.toLowerCase().includes(tagInput.value.toLowerCase())),
    )
    .slice(0, 8),
)
function addTag(value = tagInput.value) {
  const v = value.trim()
  if (!v) return
  if (tags.value.length >= 10) {
    toast('error', t('admin.editor.tagLimit'))
    return
  }
  if ([...v].length > 24) {
    toast('error', t('admin.editor.tagLimit'))
    return
  }
  if (tags.value.some((x) => x.toLowerCase() === v.toLowerCase())) {
    toast('error', t('admin.tags.rename'))
    return
  }
  tags.value.push(v)
  tagInput.value = ''
}
function restoreDraft() {
  try {
    const saved = JSON.parse(localStorage.getItem(draftKey) || 'null')
    if (saved) {
      title.value = saved.title || ''
      slug.value = saved.slug || ''
      summary.value = saved.summary || ''
      content.value = saved.content || ''
      status.value = saved.status || 'draft'
      tags.value = Array.isArray(saved.tags) ? saved.tags : []
      toast('info', t('admin.editor.draftAuto'))
    }
  } catch {}
}
function updatePreview() {
  clearTimeout(previewTimer)
  previewTimer = setTimeout(() => {
    previewHtml.value = DOMPurify.sanitize(marked.parse(content.value || ''))
  }, 180)
}
function persistDraft() {
  if (loading.value || !dirty.value) return
  try {
    localStorage.setItem(draftKey, snapshot())
    lastSavedAt.value = new Date().toLocaleTimeString('zh-CN', {
      hour: '2-digit',
      minute: '2-digit',
    })
  } catch {}
}
function scheduleDraftPersist() {
  clearTimeout(draftTimer)
  draftTimer = setTimeout(persistDraft, 700)
}
async function init() {
  try {
    knownTags.value = (await adminRequest('/api/v1/admin/tags')).items.map((x) => x.name)
  } catch {}
  if (id) {
    try {
      const a = await adminRequest(`/api/v1/admin/articles/${id}`)
      title.value = a.title
      slug.value = a.slug
      summary.value = a.summary
      content.value = a.contentMd || ''
      status.value = a.status
      tags.value = [...(a.tags || [])]
      initialSlug.value = a.slug
      initialStatus.value = a.status
    } catch (e) {
      error.value = errorMessage(e)
    }
  }
  baseline.value = snapshot()
  restoreDraft()
  loading.value = false
}
onMounted(init)
async function save() {
  if (!title.value.trim()) {
    toast('error', t('admin.editor.title'))
    return
  }
  if ([...title.value.trim()].length > 200) {
    toast('error', t('admin.editor.title'))
    return
  }
  if ([...summary.value].length > 1000) {
    toast('error', t('admin.editor.summary'))
    return
  }
  if ([...content.value].length > 200000) {
    toast('error', t('admin.editor.preview'))
    return
  }
  if (id && !slug.value.trim()) {
    toast('error', t('admin.editor.title'))
    return
  }
  if ([...slug.value].length > 200) {
    toast('error', t('admin.editor.title'))
    return
  }
  if (
    status.value !== initialStatus.value &&
    !(await confirmAction(
      status.value === 'published' ? t('admin.editor.savePublish') : t('admin.editor.saveDraft'),
      t('admin.confirm'),
    ))
  )
    return
  if (
    id &&
    initialSlug.value &&
    slug.value.trim() !== initialSlug.value &&
    !(await confirmAction(t('admin.editor.title'), t('admin.confirm')))
  )
    return
  saving.value = true
  try {
    const body = {
      title: title.value.trim(),
      summary: summary.value.trim(),
      contentMd: content.value,
      status: status.value,
      tags: tags.value,
      ...(slug.value.trim() ? { slug: slug.value.trim() } : {}),
    }
    const saved = id
      ? await adminRequest(`/api/v1/admin/articles/${id}`, { method: 'PUT', body })
      : await adminRequest('/api/v1/admin/articles', { method: 'POST', body })
    if (!id) {
      initialSlug.value = saved.slug
    }
    initialStatus.value = status.value
    baseline.value = snapshot()
    localStorage.removeItem(draftKey)
    toast(
      'success',
      status.value === 'published' ? t('admin.editor.published') : t('admin.editor.saved'),
    )
    router.push('/')
  } catch (e) {
    toast('error', e.code === 'network' ? t('admin.editor.networkDraft') : errorMessage(e))
  } finally {
    saving.value = false
  }
}
watch(content, updatePreview)
watch([title, slug, summary, content, status, tags], scheduleDraftPersist, { deep: true })
onBeforeRouteLeave(async () => {
  if (dirty.value && !saving.value) {
    persistDraft()
    return await confirmAction(t('admin.editor.leave'), t('admin.editor.confirmLeave'))
  }
})
function beforeUnload(event) {
  if (dirty.value) {
    persistDraft()
    event.preventDefault()
    event.returnValue = ''
  }
}
onMounted(() => {
  window.addEventListener('beforeunload', beforeUnload)
  updatePreview()
})
onBeforeUnmount(() => {
  window.removeEventListener('beforeunload', beforeUnload)
  clearTimeout(previewTimer)
  clearTimeout(draftTimer)
})
</script>

<template>
  <main id="admin-content" class="admin-main" tabindex="-1">
    <div class="list-head">
      <div>
        <h1>{{ id ? t('admin.editor.edit') : t('admin.editor.create') }}</h1>
        <p class="editor-save-state">
          {{
            lastSavedAt
              ? t('admin.editor.draftSaved', { time: lastSavedAt })
              : t('admin.editor.draftAuto')
          }}
        </p>
      </div>
      <div class="head-actions">
        <RouterLink class="btn btn-secondary" to="/">{{ t('web.back') }}</RouterLink
        ><button class="btn" :disabled="saving || loading" @click="save">
          {{
            saving
              ? t('admin.common.saving')
              : status === 'published'
                ? t('admin.editor.savePublish')
                : t('admin.editor.saveDraft')
          }}
        </button>
      </div>
    </div>
    <div v-if="loading" class="card skeleton sk-line">{{ t('admin.common.loading') }}</div>
    <div v-else-if="error" class="card error-card" role="alert">{{ error }}</div>
    <form v-else class="editor-form" @submit.prevent="save">
      <section class="card form-card">
        <div class="field">
          <label for="article-title"
            >{{ t('admin.editor.title') }}
            <span class="field-counter">{{ [...title].length }} / 200</span></label
          ><input
            id="article-title"
            v-model="title"
            name="title"
            :placeholder="t('admin.editor.title')"
            maxlength="200"
            required
          />
        </div>
        <div class="field">
          <label for="article-slug"
            >Slug <span class="field-hint-inline">{{ t('admin.editor.slugHint') }}</span></label
          ><input
            id="article-slug"
            v-model="slug"
            name="slug"
            :placeholder="t('admin.editor.slugPlaceholder')"
            maxlength="200"
            autocomplete="off"
          />
        </div>
        <div class="field">
          <label for="article-tags"
            >{{ t('admin.editor.tags') }}
            <span class="field-hint-inline">{{ t('admin.editor.tagLimit') }}</span></label
          >
          <div class="chips">
            <span v-for="(tag, index) in tags" :key="tag" class="chip"
              >{{ tag }}
              <button
                type="button"
                class="chip-remove"
                :aria-label="t('admin.editor.removeTag', { tag })"
                @click="tags.splice(index, 1)"
              >
                ×
              </button></span
            >
          </div>
          <div class="chip-editor">
            <input
              id="article-tags"
              v-model="tagInput"
              class="chip-input"
              name="tags"
              :placeholder="t('admin.editor.addTag')"
              autocomplete="off"
              @keydown.enter.prevent="addTag"
            /><button class="btn btn-secondary btn-sm" type="button" @click="addTag()">
              {{ t('admin.editor.addTag') }}
            </button>
          </div>
          <div v-if="suggestions.length" class="tag-suggestions">
            <button
              v-for="tag in suggestions"
              :key="tag"
              class="tag-suggestion"
              type="button"
              @click="addTag(tag)"
            >
              {{ tag }}
            </button>
          </div>
        </div>
        <div class="field">
          <label for="article-summary"
            >{{ t('admin.editor.summary') }}
            <span class="field-counter">{{ [...summary].length }} / 1000</span></label
          ><textarea
            id="article-summary"
            v-model="summary"
            name="summary"
            rows="3"
            maxlength="1000"
            :placeholder="t('admin.editor.summary')"
          ></textarea>
        </div>
        <fieldset class="field status-field">
          <legend>{{ t('admin.dashboard.statusFilter') }}</legend>
          <div class="radio-pills">
            <label
              ><input v-model="status" type="radio" name="status" value="draft" /><span>{{
                t('admin.dashboard.draft')
              }}</span></label
            ><label
              ><input v-model="status" type="radio" name="status" value="published" /><span>{{
                t('admin.dashboard.published')
              }}</span></label
            >
          </div>
        </fieldset>
      </section>
      <section class="editor-grid">
        <div class="field editor-pane">
          <label for="article-content"
            >{{ t('admin.editor.preview') }}
            <span class="field-counter"
              >{{ [...content].length.toLocaleString('zh-CN') }} / 200,000</span
            ></label
          ><textarea
            id="article-content"
            v-model="content"
            name="contentMd"
            class="markdown-input"
            placeholder="# Title&#10;&#10;Start writing in Markdown…"
            required
          ></textarea>
        </div>
        <div class="editor-pane">
          <label>{{ t('admin.editor.preview') }}</label>
          <div class="md-preview card preview-scroll" v-html="previewHtml"></div>
        </div>
      </section>
    </form>
  </main>
</template>
