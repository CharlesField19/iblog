<script setup>
defineProps({ post: { type: Object, required: true } })
</script>

<template>
  <RouterLink class="card post-card" :to="`/articles/${encodeURIComponent(post.slug)}`">
    <div class="post-meta">
      <span v-for="tag in post.tags" :key="tag" class="tag">{{ tag }}</span>
      <time :datetime="post.publishedAt || post.updatedAt">{{ (post.publishedAt || post.updatedAt || '').slice(0, 10) }}</time>
    </div>
    <h3>{{ post.title }}</h3>
    <p>{{ post.summary }}</p>
  </RouterLink>
</template>
