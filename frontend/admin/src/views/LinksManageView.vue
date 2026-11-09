<script setup lang="ts">
import { errorMessage } from '../../../shared/i18n'
import { onMounted, reactive, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { adminRequest } from '../api'
import type { Friend } from '../../../shared/contracts'
import { useToast, useConfirm } from '../injections'
import LoadingState from '../../../shared/LoadingState.vue'

const toast = useToast()
const { t } = useI18n()
const confirmAction = useConfirm()
const items = ref<Friend[]>([])
const loading = ref(false)
const saving = ref(false)
const error = ref('')
const editId = ref<number | null>(null)
const blank = () => ({ name: '', url: '', avatar: '', description: '', sortOrder: 0 })
function safeHref(value: string) {
  try {
    const url = new URL(value)
    return ['http:', 'https:'].includes(url.protocol) ? url.href : ''
  } catch {
    return ''
  }
}
const form = reactive(blank())
function reset() {
  Object.assign(form, blank())
  editId.value = null
}
async function load() {
  loading.value = true
  error.value = ''
  try {
    items.value = (await adminRequest<{ items: Friend[] }>('/api/v1/admin/friends')).items || []
  } catch (e) {
    error.value = errorMessage(e)
  } finally {
    loading.value = false
  }
}
onMounted(load)
function edit(item: Friend) {
  editId.value = item.id
  Object.assign(form, {
    name: item.name,
    url: item.url,
    avatar: item.avatar,
    description: item.description,
    sortOrder: item.sortOrder,
  })
}
async function save() {
  if (!form.name.trim() || !form.url.trim()) {
    toast('error', t('admin.links.empty'))
    return
  }
  if (
    [...form.name.trim()].length > 100 ||
    [...form.url.trim()].length > 2048 ||
    [...form.avatar.trim()].length > 2048 ||
    [...form.description].length > 1000
  ) {
    toast('error', t('admin.links.tooLong'))
    return
  }
  saving.value = true
  try {
    await adminRequest(
      editId.value ? `/api/v1/admin/friends/${editId.value}` : '/api/v1/admin/friends',
      {
        method: editId.value ? 'PUT' : 'POST',
        body: {
          ...form,
          name: form.name.trim(),
          url: form.url.trim(),
          avatar: form.avatar.trim(),
          description: form.description.trim(),
        },
      },
    )
    toast('success', editId.value ? t('admin.links.saved') : t('admin.links.added'))
    reset()
    await load()
  } catch (e) {
    toast('error', errorMessage(e))
  } finally {
    saving.value = false
  }
}
async function remove(item: Friend) {
  if (
    !(await confirmAction(
      t('admin.links.deleteConfirm', { name: item.name }),
      t('admin.links.deleteTitle'),
    ))
  )
    return
  try {
    await adminRequest(`/api/v1/admin/friends/${item.id}`, { method: 'DELETE' })
    toast('success', t('admin.links.deleted'))
    load()
  } catch (e) {
    toast('error', errorMessage(e))
  }
}
</script>

<template>
  <main id="admin-content" class="admin-main" tabindex="-1">
    <div class="list-head">
      <h1>{{ t('admin.links.title') }}</h1>
    </div>
    <form class="card form-card friend-form" @submit.prevent="save">
      <h2 class="form-title">{{ editId ? t('admin.links.save') : t('admin.links.add') }}</h2>
      <div class="field-row">
        <div class="field">
          <label for="friend-name">{{ t('admin.links.name') }}</label
          ><input
            id="friend-name"
            v-model="form.name"
            name="name"
            maxlength="100"
            :placeholder="t('admin.links.namePlaceholder')"
            required
          />
        </div>
        <div class="field">
          <label for="friend-url">{{ t('admin.links.url') }}</label
          ><input
            id="friend-url"
            v-model="form.url"
            name="url"
            type="url"
            placeholder="https://example.com…"
            required
          />
        </div>
      </div>
      <div class="field-row">
        <div class="field">
          <label for="friend-avatar">{{ t('admin.links.avatar') }}</label
          ><input
            id="friend-avatar"
            v-model="form.avatar"
            name="avatar"
            type="url"
            maxlength="2048"
            :placeholder="t('admin.links.avatarPlaceholder')"
          />
        </div>
        <div class="field">
          <label for="friend-sort">{{ t('admin.links.sort') }}</label
          ><input id="friend-sort" v-model.number="form.sortOrder" name="sortOrder" type="number" />
        </div>
      </div>
      <div class="field">
        <label for="friend-description">{{ t('admin.links.intro') }}</label
        ><input
          id="friend-description"
          v-model="form.description"
          name="description"
          maxlength="1000"
          :placeholder="t('admin.links.introPlaceholder')"
        />
      </div>
      <div class="form-actions">
        <button class="btn" type="submit" :disabled="saving">
          <LoadingState v-if="saving" variant="inline" />
          <span>{{
            saving
              ? editId
                ? t('admin.common.saving')
                : t('admin.links.adding')
              : editId
                ? t('admin.links.save')
                : t('admin.links.add')
          }}</span></button
        ><button v-if="editId" class="btn btn-secondary" type="button" @click="reset">
          {{ t('admin.common.cancel') }}
        </button>
      </div>
    </form>
    <p v-if="error" class="card error-card" role="alert">
      {{ error }} <button class="link-btn" @click="load">{{ t('admin.common.retry') }}</button>
    </p>
    <div v-else-if="loading" class="card">
      <LoadingState variant="card" :label="t('admin.common.loading')" />
    </div>
    <div v-else-if="!items.length" class="card empty-state">{{ t('admin.links.emptyList') }}</div>
    <div v-else class="stack">
      <section v-for="item in items" :key="item.id" class="card admin-row friend-row">
        <img
          v-if="item.avatar && safeHref(item.avatar)"
          class="avatar"
          :src="safeHref(item.avatar)"
          alt=""
          width="44"
          height="44"
          loading="lazy"
        /><span v-else class="avatar avatar-fallback" aria-hidden="true">{{
          item.name.slice(0, 1)
        }}</span>
        <div class="row-main">
          <h3 class="row-title">{{ item.name }}</h3>
          <p class="row-meta">
            <a
              v-if="safeHref(item.url)"
              :href="safeHref(item.url)"
              target="_blank"
              rel="noopener noreferrer"
              >{{ item.url }}</a
            ><span v-else>{{ item.url }}</span
            ><span v-if="item.description"> · {{ item.description }}</span> ·
            {{ t('admin.links.sortLabel') }}
            {{ item.sortOrder }}
          </p>
        </div>
        <div class="row-actions">
          <button class="link-btn" @click="edit(item)">{{ t('admin.links.edit') }}</button
          ><button class="link-btn danger" @click="remove(item)">
            {{ t('admin.common.delete') }}
          </button>
        </div>
      </section>
    </div>
  </main>
</template>
