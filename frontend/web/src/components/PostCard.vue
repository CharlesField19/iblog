<script setup lang="ts">
import type { ArticleSummary } from '../../../shared/contracts'
import { useI18n } from 'vue-i18n'
import vReveal from '../directives/reveal'
defineProps<{ post: ArticleSummary }>()
const { locale } = useI18n()
function formatDate(value?: string | null) {
  if (!value) return ''
  return new Intl.DateTimeFormat(locale.value, {
    year: 'numeric',
    month: 'short',
    day: 'numeric',
  }).format(new Date(value))
}
</script>

<template>
  <RouterLink v-reveal class="card post-card" :to="`/articles/${encodeURIComponent(post.slug)}`">
    <div class="post-meta">
      <span v-for="tag in post.tags" :key="tag" class="tag">{{ tag }}</span>
      <time :datetime="post.publishedAt || post.updatedAt">{{
        formatDate(post.publishedAt || post.updatedAt)
      }}</time>
    </div>
    <h3>{{ post.title }}</h3>
    <p>{{ post.summary }}</p>
  </RouterLink>
</template>
