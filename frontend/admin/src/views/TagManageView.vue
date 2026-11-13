<script setup lang="ts">
import { errorMessage } from '../../../shared/i18n'
import { onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { adminRequest } from '../api'
import type { Tag } from '../../../shared/contracts'
import { useToast, useConfirm } from '../injections'
import LoadingState from '../../../shared/LoadingState.vue'

const toast = useToast()
const confirmAction = useConfirm()
const { t } = useI18n()
const items = ref<Tag[]>([])
const name = ref('')
const editId = ref<number | null>(null)
const editName = ref('')
const loading = ref(false)
const error = ref('')
async function load() {
  loading.value = true
  error.value = ''
  try {
    items.value = (await adminRequest<{ items: Tag[] }>('/api/v1/admin/tags')).items || []
  } catch (e) {
    error.value = errorMessage(e)
  } finally {
    loading.value = false
  }
}
onMounted(load)
async function create() {
  if (!name.value.trim()) {
    toast('error', t('admin.tags.emptyName'))
    return
  }
  try {
    await adminRequest('/api/v1/admin/tags', { method: 'POST', body: { name: name.value.trim() } })
    name.value = ''
    toast('success', t('admin.tags.createSuccess'))
    load()
  } catch (e) {
    toast('error', errorMessage(e))
  }
}
function startEdit(item: Tag) {
  editId.value = item.id
  editName.value = item.name
}
async function saveEdit() {
  const current = items.value.find((item) => item.id === editId.value)
  if (!editName.value.trim()) {
    toast('error', t('admin.tags.emptyName'))
    return
  }
  if (
    current?.count &&
    !(await confirmAction(
      t('admin.tags.renameConfirm', { count: current.count, name: editName.value.trim() }),
      t('admin.tags.confirmRename'),
    ))
  )
    return
  try {
    await adminRequest(`/api/v1/admin/tags/${editId.value}`, {
      method: 'PUT',
      body: { name: editName.value.trim() },
    })
    editId.value = null
    toast('success', t('admin.tags.updated'))
    load()
  } catch (e) {
    toast('error', errorMessage(e))
  }
}
async function remove(item: Tag) {
  if (
    !(await confirmAction(
      item.count
        ? t('admin.tags.deleteUsed', { name: item.name, count: item.count })
        : t('admin.tags.deleteUnused', { name: item.name }),
      t('admin.tags.deleteTitle'),
    ))
  )
    return
  try {
    await adminRequest(`/api/v1/admin/tags/${item.id}`, { method: 'DELETE' })
    toast('success', t('admin.tags.deleted'))
    load()
  } catch (e) {
    toast('error', errorMessage(e))
  }
}
</script>

<template>
  <main id="admin-content" class="admin-main" tabindex="-1">
    <div class="list-head">
      <h1>{{ t('admin.tags.title') }}</h1>
    </div>
    <section class="card tag-form">
      <h2 class="form-title">{{ t('admin.tags.create') }}</h2>
      <form class="inline-form" @submit.prevent="create">
        <input
          v-model="name"
          class="chip-input"
          maxlength="24"
          :placeholder="t('admin.tags.example')"
          :aria-label="t('admin.tags.newName')"
          autocomplete="off"
        /><button class="btn" type="submit">{{ t('admin.common.add') }}</button>
      </form>
    </section>
    <p v-if="error" class="card error-card" role="alert">
      {{ error }}
      <button class="link-btn" type="button" @click="load">{{ t('admin.common.retry') }}</button>
    </p>
    <div v-else-if="loading" class="card">
      <LoadingState variant="card" :label="t('admin.common.loading')" />
    </div>
    <div v-else-if="!items.length" class="card empty-state">{{ t('admin.tags.empty') }}</div>
    <div v-else class="stack">
      <section v-for="item in items" :key="item.id" class="card admin-row tag-row">
        <div class="row-main">
          <template v-if="editId === item.id"
            ><form class="inline-form" @submit.prevent="saveEdit">
              <input
                v-model="editName"
                class="chip-input"
                maxlength="24"
                :aria-label="t('admin.tags.editName')"
              /><button class="btn btn-sm" type="submit">{{ t('admin.common.save') }}</button
              ><button class="btn btn-secondary btn-sm" type="button" @click="editId = null">
                {{ t('admin.common.cancel') }}
              </button>
            </form></template
          ><template v-else
            ><h3 class="row-title">{{ item.name }}</h3>
            <span class="tag-unused">{{
              t('admin.tags.count', { count: item.count })
            }}</span></template
          >
        </div>
        <div v-if="editId !== item.id" class="row-actions">
          <button class="link-btn" type="button" @click="startEdit(item)">
            {{ t('admin.tags.edit') }}</button
          ><button class="link-btn danger" type="button" @click="remove(item)">
            {{ t('admin.common.delete') }}
          </button>
        </div>
      </section>
    </div>
  </main>
</template>
