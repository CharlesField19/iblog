<script setup>
import { onMounted, ref } from 'vue'
import { request } from '../../../shared/api.js'

const friends = ref([]); const error = ref(''); const loading = ref(true)
async function load() {
  loading.value = true; error.value = ''
  try { friends.value = (await request('/api/v1/friends')).items || [] }
  catch (e) { error.value = e.message }
  finally { loading.value = false }
}
onMounted(load)
</script>

<template>
  <section class="page-head"><div class="container"><h1>友链</h1><p class="sub">一些常去逛的地方，也欢迎你加入。想交换友链？<RouterLink to="/about">给我写信</RouterLink>。</p></div></section>
  <section class="section"><div class="container friends-grid">
    <p v-if="loading">正在加载…</p>
    <div v-else-if="error" role="alert"><p>{{ error }}</p><button class="btn" @click="load">重试</button></div>
    <template v-else><a v-for="friend in friends" :key="friend.id" class="card friend-card" :href="friend.url" target="_blank" rel="noopener noreferrer"><img v-if="friend.avatar" class="friend-ava friend-ava-img" :src="friend.avatar" :alt="friend.name"><span v-else class="friend-ava friend-ava-fallback">{{ friend.name.slice(0,1) }}</span><span><strong class="friend-name">{{ friend.name }}</strong><span class="friend-desc">{{ friend.description }}</span></span></a><p v-if="!friends.length" class="muted">还没有友链。</p></template>
  </div></section>
</template>
