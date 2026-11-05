<script setup>
defineProps({ post: { type: Object, required: true } })
function formatDate(value) {
  if (!value) return ''
  return new Intl.DateTimeFormat('zh-CN', { year: 'numeric', month: 'short', day: 'numeric' }).format(new Date(value))
}
</script>

<template>
  <RouterLink class="card post-card" :to="`/articles/${encodeURIComponent(post.slug)}`">
    <div class="post-meta">
      <span v-for="tag in post.tags" :key="tag" class="tag">{{ tag }}</span>
      <time :datetime="post.publishedAt || post.updatedAt">{{ formatDate(post.publishedAt || post.updatedAt) }}</time>
    </div>
    <h3>{{ post.title }}</h3>
    <p>{{ post.summary }}</p>
  </RouterLink>
</template>
