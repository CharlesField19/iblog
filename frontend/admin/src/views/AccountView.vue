<script setup lang="ts">
import { errorMessage } from '../../../shared/i18n'
import { ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { adminRequest } from '../api'
import type { SessionResponse } from '../../../shared/contracts'
import { useToast, useConfirm } from '../injections'
import { saveSession } from '../session'

const toast = useToast()
const { t } = useI18n()
const username = ref('')
const current = ref('')
const next = ref('')
const confirmPassword = ref('')
const savingName = ref(false)
const savingPassword = ref(false)
async function changeUsername() {
  const value = username.value.trim()
  if (!/^[A-Za-z0-9_-]{3,32}$/.test(value)) {
    toast('error', t('admin.account.usernameRule'))
    return
  }
  savingName.value = true
  try {
    const res = await adminRequest<SessionResponse>('/api/v1/admin/account/username', {
      method: 'PUT',
      body: { username: value },
    })
    saveSession(res)
    toast('success', t('admin.account.usernameSaved'))
    username.value = ''
  } catch (e) {
    toast('error', errorMessage(e))
  } finally {
    savingName.value = false
  }
}
async function changePassword() {
  if (next.value.length < 8) {
    toast('error', t('admin.account.passwordShort'))
    return
  }
  if (next.value !== confirmPassword.value) {
    toast('error', t('admin.account.passwordMismatch'))
    return
  }
  savingPassword.value = true
  try {
    const res = await adminRequest<SessionResponse>('/api/v1/admin/account/password', {
      method: 'PUT',
      body: { currentPassword: current.value, newPassword: next.value },
    })
    saveSession(res)
    toast('success', t('admin.account.passwordSaved'))
    current.value = ''
    next.value = ''
    confirmPassword.value = ''
  } catch (e) {
    toast('error', errorMessage(e))
  } finally {
    savingPassword.value = false
  }
}
</script>

<template>
  <main id="admin-content" class="admin-main" tabindex="-1">
    <div class="list-head">
      <h1>{{ t('admin.account.title') }}</h1>
    </div>
    <div class="stack">
      <section class="card section-card">
        <h2 class="section-title">{{ t('admin.account.editUsername') }}</h2>
        <form class="account-form" @submit.prevent="changeUsername">
          <div class="field">
            <label for="new-username">{{ t('admin.account.newUsername') }}</label
            ><input
              id="new-username"
              v-model="username"
              autocomplete="username"
              :placeholder="t('admin.account.usernameRule')"
              required
            />
          </div>
          <p class="section-hint">{{ t('admin.account.usernameHint') }}</p>
          <button class="btn" :disabled="savingName">
            {{ savingName ? t('admin.common.saving') : t('admin.account.saveUsername') }}
          </button>
        </form>
      </section>
      <section class="card section-card">
        <h2 class="section-title">{{ t('admin.account.editPassword') }}</h2>
        <form class="account-form" @submit.prevent="changePassword">
          <div class="field">
            <label for="current-password">{{ t('admin.account.currentPassword') }}</label
            ><input
              id="current-password"
              v-model="current"
              type="password"
              autocomplete="current-password"
              required
            />
          </div>
          <div class="field">
            <label for="new-password">{{ t('admin.account.newPassword') }}</label
            ><input
              id="new-password"
              v-model="next"
              type="password"
              autocomplete="new-password"
              minlength="8"
              required
            />
          </div>
          <p class="section-hint">{{ t('admin.account.passwordHint') }}</p>
          <div class="field">
            <label for="confirm-password">{{ t('admin.account.confirmPassword') }}</label
            ><input
              id="confirm-password"
              v-model="confirmPassword"
              type="password"
              autocomplete="new-password"
              required
            />
          </div>
          <button class="btn" :disabled="savingPassword">
            {{ savingPassword ? t('admin.common.saving') : t('admin.account.savePassword') }}
          </button>
        </form>
      </section>
    </div>
  </main>
</template>
