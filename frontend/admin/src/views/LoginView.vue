<script setup lang="ts">
import { errorMessage } from '../../../shared/i18n'
import { ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { useRoute, useRouter } from 'vue-router'
import { adminRequest } from '../api'
import { useToast, useConfirm } from '../injections'
import { saveSession } from '../session'
import { request } from '../../../shared/api'
import type { SessionResponse } from '../../../shared/contracts'
import LoadingState from '../../../shared/LoadingState.vue'

const router = useRouter()
const { t } = useI18n()
const route = useRoute()
const toast = useToast()
const username = ref('')
const password = ref('')
const error = ref('')
const loading = ref(false)
const setupRequired = ref(false)
const checking = ref(true)
request<{ setupRequired: boolean }>('/api/v1/auth/setup-status')
  .then((data) => {
    setupRequired.value = Boolean(data.setupRequired)
  })
  .catch((e) => {
    error.value = errorMessage(e)
  })
  .finally(() => {
    checking.value = false
  })
async function submit() {
  if (loading.value) return
  if (!username.value.trim() || !password.value) {
    error.value = t('admin.login.missingCredentials')
    return
  }
  if (setupRequired.value && password.value.length < 8) {
    error.value = t('admin.login.passwordTooShort')
    return
  }
  loading.value = true
  error.value = ''
  try {
    const response = setupRequired.value
      ? await request<SessionResponse>('/api/v1/auth/setup', {
          method: 'POST',
          body: { username: username.value.trim(), password: password.value },
        })
      : await adminRequest<SessionResponse>('/api/v1/auth/login', {
          method: 'POST',
          body: { username: username.value.trim(), password: password.value },
        })
    saveSession(response)
    toast('success', setupRequired.value ? t('admin.login.created') : t('admin.login.success'))
    router.replace(typeof route.query.next === 'string' ? route.query.next : '/')
  } catch (e) {
    error.value = errorMessage(e)
  } finally {
    loading.value = false
  }
}
</script>

<template>
  <main id="admin-content" class="login-wrap" tabindex="-1">
    <section class="login-card">
      <h1>{{ setupRequired ? t('admin.login.setup') : t('admin.login.title') }}</h1>
      <p v-if="setupRequired" class="setup-copy">{{ t('admin.login.firstUse') }}</p>
      <LoadingState v-if="checking" variant="card" :label="t('admin.login.checking')" />
      <form v-else @submit.prevent="submit">
        <div class="field">
          <label for="username">{{ t('admin.login.username') }}</label
          ><input
            id="username"
            v-model="username"
            name="username"
            autocomplete="username"
            spellcheck="false"
            :placeholder="t('admin.login.usernamePlaceholder')"
            :aria-invalid="error ? 'true' : undefined"
            :aria-describedby="error ? 'login-error' : undefined"
            required
          />
        </div>
        <div class="field">
          <label for="password">{{
            setupRequired ? t('admin.login.setPassword') : t('admin.login.password')
          }}</label
          ><input
            id="password"
            v-model="password"
            name="password"
            type="password"
            :autocomplete="setupRequired ? 'new-password' : 'current-password'"
            :placeholder="
              setupRequired ? t('admin.login.minPassword') : t('admin.login.passwordPlaceholder')
            "
            :minlength="setupRequired ? 8 : undefined"
            :aria-invalid="error ? 'true' : undefined"
            :aria-describedby="error ? 'login-error' : undefined"
            required
          />
        </div>
        <p v-if="error" id="login-error" class="login-error" role="alert">{{ error }}</p>
        <button class="btn login-btn" :disabled="loading">
          <LoadingState v-if="loading" variant="inline" />
          <span>{{
            loading
              ? setupRequired
                ? t('admin.login.creating')
                : t('admin.login.loggingIn')
              : setupRequired
                ? t('admin.login.createAccount')
                : t('admin.login.signIn')
          }}</span>
        </button>
      </form>
    </section>
  </main>
</template>
