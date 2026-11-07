<script setup lang="ts">
import { errorMessage } from '../../../shared/i18n'
import { onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { getPublicData } from '../../../shared/publicData'
import type { Friend } from '../../../shared/contracts'
import FriendCardSkeleton from '../components/FriendCardSkeleton.vue'

const friends = ref<Friend[]>([])
const error = ref('')
const loading = ref(true)
const { t } = useI18n()
function safeHref(value: string) {
  try {
    const url = new URL(value)
    return ['http:', 'https:'].includes(url.protocol) ? url.href : ''
  } catch {
    return ''
  }
}
async function load() {
  loading.value = true
  error.value = ''
  try {
    friends.value = (await getPublicData<{ items: Friend[] }>('/api/v1/friends')).items || []
  } catch (e) {
    error.value = errorMessage(e)
  } finally {
    loading.value = false
  }
}
onMounted(load)
</script>

<template>
  <section class="page-head">
    <div class="page-container">
      <h1>{{ t('web.friends') }}</h1>
      <p class="sub">
        {{ t('web.friendsInvite') }} <RouterLink to="/about">{{ t('web.contact') }}</RouterLink
        >。
      </p>
    </div>
  </section>
  <section class="section">
    <div class="page-container friends-grid">
      <div v-if="loading" class="friends-grid"><FriendCardSkeleton v-for="i in 3" :key="i" /></div>
      <div v-else-if="error" role="alert">
        <p>{{ error }}</p>
        <button class="btn" @click="load">{{ t('web.retry') }}</button>
      </div>
      <template v-else
        ><a
          v-for="friend in friends"
          :key="friend.id"
          class="card friend-card"
          :href="safeHref(friend.url) || undefined"
          :aria-disabled="!safeHref(friend.url) || undefined"
          target="_blank"
          rel="noopener noreferrer"
          ><img
            v-if="friend.avatar && safeHref(friend.avatar)"
            class="friend-ava friend-ava-img"
            :src="safeHref(friend.avatar)"
            alt=""
            width="52"
            height="52"
            loading="lazy"
          /><span v-else class="friend-ava friend-ava-fallback" aria-hidden="true">{{
            friend.name.slice(0, 1)
          }}</span
          ><span
            ><strong class="friend-name">{{ friend.name }}</strong
            ><span class="friend-desc">{{ friend.description || friend.url }}</span></span
          ></a
        >
        <p v-if="!friends.length" class="muted">{{ t('web.noFriends') }}</p></template
      >
    </div>
  </section>
</template>
