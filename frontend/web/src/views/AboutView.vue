<script setup>
import { onMounted, ref } from 'vue'
import { request } from '../../../shared/api.js'

const about = ref({ intro: '', bio: '', skills: [], timeline: [], contactNote: '' })
const contacts = ref({})
const social = [
  { key: 'github', label: 'GitHub', icon: '<path d="M9 19c-4.3 1.4-4.3-2.5-6-3m12 6v-3.9a3.4 3.4 0 0 0-.9-2.7c3-.3 6.2-1.5 6.2-6.7a5.2 5.2 0 0 0-1.4-3.6 4.8 4.8 0 0 0-.1-3.6S17.7 1.2 15 3a13.4 13.4 0 0 0-7 0C5.3 1.2 4.2 1.5 4.2 1.5a4.8 4.8 0 0 0-.1 3.6 5.2 5.2 0 0 0-1.4 3.6c0 5.2 3.2 6.4 6.2 6.7A3.4 3.4 0 0 0 8 18v4"/>' },
  { key: 'x', label: 'X', icon: '<path d="M4 4l16 16M20 4 4 20"/>' },
  { key: 'telegram', label: 'Telegram', icon: '<path d="m22 2-7 20-4-9-9-4 20-7ZM22 2 11 13"/>' },
]
onMounted(async () => {
  try {
    const settings = await request('/api/v1/settings')
    about.value = { ...about.value, ...(settings.about || {}) }
    contacts.value = settings.contacts || {}
  } catch { /* leave the page usable if settings are unavailable */ }
})
</script>

<template>
  <section class="page-head"><div class="container"><h1>关于</h1><p class="sub">{{ about.intro || '关于这个博客' }}</p></div></section>
  <section class="section"><div class="container about-grid">
    <div class="card"><h2 class="section-title">我是谁</h2><p>{{ about.bio || '在后台的“关于管理”中添加介绍。' }}</p><h3 v-if="about.skills?.length">常用技能</h3><div class="skill-list"><span v-for="skill in about.skills" :key="skill" class="chip">{{ skill }}</span></div></div>
    <div class="card"><h2 class="section-title">时间线</h2><div class="timeline"><div v-for="(item, index) in about.timeline" :key="index" class="t-item"><span class="t-year">{{ item.year }}</span><h4>{{ item.title }}</h4><p>{{ item.description }}</p></div><p v-if="!about.timeline?.length" class="muted">尚未添加经历。</p></div></div>
  </div></section>
  <section class="section"><div class="container"><div class="card contact-card"><h2 class="section-title">联系我</h2><p>{{ about.contactNote || '欢迎交流与反馈，通常 24 小时内回复。' }}</p><a v-if="contacts.email" class="btn email-button" :href="`mailto:${contacts.email}`">{{ contacts.email }}</a><div class="social-icons"><a v-for="site in social" :key="site.key" v-show="contacts[site.key]" :href="contacts[site.key]" :aria-label="site.label" target="_blank" rel="noopener noreferrer"><svg viewBox="0 0 24 24" aria-hidden="true" v-html="site.icon"></svg></a></div></div></div></section>
</template>
