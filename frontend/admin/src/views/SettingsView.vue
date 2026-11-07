<script setup lang="ts">
import { errorMessage } from '../../../shared/i18n'
import { onMounted, reactive, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { adminRequest } from '../api'
import type { AdminSettings } from '../../../shared/contracts'
import { useToast, useConfirm } from '../injections'

const toast = useToast()
const { t } = useI18n()
const contacts = reactive({ email: '', github: '', x: '', telegram: '' })
const loading = ref(true)
const saving = ref(false)
const error = ref('')
const site = reactive({
  name: 'iBlog',
  titlePrefix: "Chenghao's",
  titleAccent: 'Blog',
  subtitle: 'Code · Life · Ideas',
  footerName: 'Chenghao',
  footerText: '用代码与热爱搭建',
})
async function load() {
  loading.value = true
  error.value = ''
  try {
    const data = await adminRequest<AdminSettings>('/api/v1/admin/settings')
    Object.assign(contacts, {
      email: '',
      github: '',
      x: '',
      telegram: '',
      ...(data.settings?.contacts || {}),
    })
    Object.assign(site, {
      name: 'iBlog',
      titlePrefix: "Chenghao's",
      titleAccent: 'Blog',
      subtitle: 'Code · Life · Ideas',
      footerName: 'Chenghao',
      footerText: '用代码与热爱搭建',
      ...(data.settings?.site || {}),
    })
  } catch (e) {
    error.value = errorMessage(e)
  } finally {
    loading.value = false
  }
}
onMounted(load)
async function save() {
  const email = contacts.email.trim()
  if (email && !email.includes('@')) {
    toast('error', t('admin.settings.invalidEmail'))
    return
  }
  for (const [label, value] of [
    ['GitHub', contacts.github],
    ['X', contacts.x],
    ['Telegram', contacts.telegram],
  ])
    if (value && !/^https?:\/\//i.test(value)) {
      toast('error', t('admin.settings.invalidUrl', { label }))
      return
    }
  saving.value = true
  try {
    await Promise.all([
      adminRequest('/api/v1/admin/settings/contacts', {
        method: 'PUT',
        body: Object.fromEntries(Object.entries(contacts).map(([k, v]) => [k, v.trim()])),
      }),
      adminRequest('/api/v1/admin/settings/site', {
        method: 'PUT',
        body: Object.fromEntries(Object.entries(site).map(([k, v]) => [k, v.trim()])),
      }),
    ])
    document.title = site.name.trim() || 'iBlog'
    toast('success', t('admin.settings.saved'))
  } catch (e) {
    toast('error', errorMessage(e))
  } finally {
    saving.value = false
  }
}
</script>

<template>
  <main id="admin-content" class="admin-main" tabindex="-1">
    <div class="list-head">
      <h1>{{ t('admin.settings.title') }}</h1>
      <button class="btn" :disabled="saving || loading" @click="save">
        {{ saving ? t('admin.common.saving') : t('admin.common.save') }}
      </button>
    </div>
    <p class="page-intro">{{ t('admin.settings.intro') }}</p>
    <p v-if="error" class="card error-card" role="alert">
      {{ error }} <button class="link-btn" @click="load">{{ t('admin.common.retry') }}</button>
    </p>
    <div v-else-if="loading" class="card skeleton">{{ t('admin.common.loading') }}</div>
    <section v-else class="card section-card">
      <h2 class="section-title">{{ t('admin.settings.homepage') }}</h2>
      <p class="page-intro">{{ t('admin.settings.homepageHint') }}</p>
      <div class="field">
        <label for="site-name">{{ t('admin.settings.siteName') }}</label
        ><input
          id="site-name"
          v-model="site.name"
          name="siteName"
          maxlength="80"
          :placeholder="t('admin.settings.exampleSite')"
          autocomplete="off"
        />
      </div>
      <div class="field">
        <label for="site-title-prefix">{{ t('admin.settings.titlePrefix') }}</label
        ><input
          id="site-title-prefix"
          v-model="site.titlePrefix"
          name="titlePrefix"
          maxlength="80"
          :placeholder="t('admin.settings.examplePrefix')"
          autocomplete="off"
        />
      </div>
      <div class="field">
        <label for="site-title-accent">{{ t('admin.settings.titleAccent') }}</label
        ><input
          id="site-title-accent"
          v-model="site.titleAccent"
          name="titleAccent"
          maxlength="80"
          :placeholder="t('admin.settings.exampleAccent')"
          autocomplete="off"
        />
      </div>
      <div class="field">
        <label for="site-subtitle">{{ t('admin.settings.subtitle') }}</label
        ><input
          id="site-subtitle"
          v-model="site.subtitle"
          name="subtitle"
          maxlength="160"
          :placeholder="t('admin.settings.exampleSubtitle')"
          autocomplete="off"
        />
      </div>
      <div class="field">
        <label for="site-footer-name">{{ t('admin.settings.footerName') }}</label
        ><input
          id="site-footer-name"
          v-model="site.footerName"
          name="footerName"
          maxlength="80"
          :placeholder="t('admin.settings.footerNamePlaceholder')"
          autocomplete="off"
        />
      </div>
      <div class="field">
        <label for="site-footer-text">{{ t('admin.settings.footerText') }}</label
        ><input
          id="site-footer-text"
          v-model="site.footerText"
          name="footerText"
          maxlength="160"
          :placeholder="t('admin.settings.footerTextPlaceholder')"
          autocomplete="off"
        />
      </div>
    </section>
    <section v-if="!loading && !error" class="card section-card">
      <h2 class="section-title">{{ t('admin.settings.contacts') }}</h2>
      <div class="field">
        <label for="contact-email">{{ t('admin.settings.email') }}</label
        ><input
          id="contact-email"
          v-model="contacts.email"
          name="email"
          type="email"
          autocomplete="email"
          placeholder="name@example.com…"
        />
      </div>
      <div class="field">
        <label for="contact-github">GitHub URL</label
        ><input
          id="contact-github"
          v-model="contacts.github"
          name="github"
          type="url"
          placeholder="https://github.com/username…"
          autocomplete="url"
        />
      </div>
      <div class="field">
        <label for="contact-x">X URL</label
        ><input
          id="contact-x"
          v-model="contacts.x"
          name="x"
          type="url"
          placeholder="https://x.com/username…"
          autocomplete="url"
        />
      </div>
      <div class="field">
        <label for="contact-telegram">Telegram URL</label
        ><input
          id="contact-telegram"
          v-model="contacts.telegram"
          name="telegram"
          type="url"
          placeholder="https://t.me/username…"
          autocomplete="url"
        />
      </div>
    </section>
  </main>
</template>
