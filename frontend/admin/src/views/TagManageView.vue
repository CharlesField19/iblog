<script setup>
import { inject, onMounted, ref } from 'vue'
import { adminRequest } from '../api.js'

const toast = inject('toast')
const confirmAction = inject('confirmAction')
const items = ref([])
const name = ref('')
const editId = ref(null)
const editName = ref('')
const loading = ref(false)
const error = ref('')
async function load() {
  loading.value = true
  error.value = ''
  try {
    items.value = (await adminRequest('/api/v1/admin/tags')).items || []
  } catch (e) {
    error.value = e.message
  } finally {
    loading.value = false
  }
}
onMounted(load)
async function create() {
  if (!name.value.trim()) {
    toast('error', '标签名不能为空')
    return
  }
  try {
    await adminRequest('/api/v1/admin/tags', { method: 'POST', body: { name: name.value.trim() } })
    name.value = ''
    toast('success', '标签已创建')
    load()
  } catch (e) {
    toast('error', e.message)
  }
}
function startEdit(item) {
  editId.value = item.id
  editName.value = item.name
}
async function saveEdit() {
  const current = items.value.find((item) => item.id === editId.value)
  if (!editName.value.trim()) {
    toast('error', '标签名不能为空')
    return
  }
  if (
    current?.count &&
    !(await confirmAction(
      `重命名后，${current.count} 篇文章会统一显示新标签「${editName.value.trim()}」。文章内容不会改变。`,
      '确认重命名标签',
    ))
  )
    return
  try {
    await adminRequest(`/api/v1/admin/tags/${editId.value}`, {
      method: 'PUT',
      body: { name: editName.value.trim() },
    })
    editId.value = null
    toast('success', '标签已更新')
    load()
  } catch (e) {
    toast('error', e.message)
  }
}
async function remove(item) {
  if (
    !(await confirmAction(
      item.count
        ? `删除标签「${item.name}」后，${item.count} 篇文章将失去此标签；文章不会删除。`
        : `删除未使用的标签「${item.name}」？`,
      '删除标签',
    ))
  )
    return
  try {
    await adminRequest(`/api/v1/admin/tags/${item.id}`, { method: 'DELETE' })
    toast('success', '标签已删除')
    load()
  } catch (e) {
    toast('error', e.message)
  }
}
</script>

<template>
  <main id="admin-content" class="admin-main" tabindex="-1">
    <div class="list-head"><h1>标签管理</h1></div>
    <section class="card tag-form">
      <h2 class="form-title">新建标签</h2>
      <form class="inline-form" @submit.prevent="create">
        <input
          v-model="name"
          class="chip-input"
          maxlength="24"
          placeholder="例如：Go…"
          aria-label="新标签名称"
          autocomplete="off"
        /><button class="btn" type="submit">添加</button>
      </form>
    </section>
    <p v-if="error" class="card error-card" role="alert">
      {{ error }} <button class="link-btn" @click="load">重试</button>
    </p>
    <div v-else-if="loading" class="card skeleton">加载中…</div>
    <div v-else-if="!items.length" class="card empty-state">还没有标签。</div>
    <div v-else class="stack">
      <section v-for="item in items" :key="item.id" class="card admin-row tag-row">
        <div class="row-main">
          <template v-if="editId === item.id"
            ><form class="inline-form" @submit.prevent="saveEdit">
              <input
                v-model="editName"
                class="chip-input"
                maxlength="24"
                aria-label="编辑标签名"
              /><button class="btn btn-sm">保存</button
              ><button class="btn btn-secondary btn-sm" type="button" @click="editId = null">
                取消
              </button>
            </form></template
          ><template v-else
            ><h3 class="row-title">{{ item.name }}</h3>
            <span class="tag-unused">{{ item.count }} 篇文章</span></template
          >
        </div>
        <div v-if="editId !== item.id" class="row-actions">
          <button class="link-btn" @click="startEdit(item)">编辑</button
          ><button class="link-btn danger" @click="remove(item)">删除</button>
        </div>
      </section>
    </div>
  </main>
</template>
