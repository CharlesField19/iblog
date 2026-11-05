<script setup>
import { computed, ref, watch } from 'vue'
import { useRoute } from 'vue-router'
import { marked } from 'marked'
import DOMPurify from 'dompurify'
import { request } from '../../../shared/api.js'

const route = useRoute(); const post = ref(null); const error = ref(''); const loading = ref(true)
const html = computed(() => DOMPurify.sanitize(marked.parse(post.value?.contentMd || '')))
async function load(slug) {
  loading.value = true; error.value = ''; post.value = null
  try { post.value = await request(`/api/v1/articles/${encodeURIComponent(slug)}`) }
  catch (e) { error.value = e.message }
  finally { loading.value = false }
}
watch(() => route.params.slug, load, { immediate: true })
</script>

<template>
  <section class="section post-page"><div class="container">
    <RouterLink class="back-link" to="/articles">← 返回文章</RouterLink>
    <p v-if="loading">正在加载文章…</p>
    <div v-else-if="error" role="alert"><p>{{ error }}</p><RouterLink to="/articles">查看其他文章</RouterLink></div>
    <article v-else-if="post">
      <div class="post-meta"><span v-for="tag in post.tags" :key="tag" class="tag">{{ tag }}</span><time :datetime="post.publishedAt || post.updatedAt">{{ (post.publishedAt || post.updatedAt || '').slice(0,10) }}</time></div>
      <h1 class="post-title">{{ post.title }}</h1><p class="post-summary">{{ post.summary }}</p>
      <div class="markdown-body" v-html="html"></div>
    </article>
  </div></section>
</template>
