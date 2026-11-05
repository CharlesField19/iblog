<script setup>
import { onMounted, ref } from 'vue'
import { request } from '../../../shared/api.js'

const about = ref({ intro: '', bio: '', skills: [], timeline: [], contactNote: '' })
const contacts = ref({})
const social = [
  { key: 'github', label: 'GitHub', icon: '<path d="M9 19c-5 1.5-5-2.5-7-3m14 6v-3.87a3.37 3.37 0 0 0-.94-2.61c3.14-.35 6.44-1.54 6.44-7A5.44 5.44 0 0 0 20 4.77 5.07 5.07 0 0 0 19.91 1S18.73.65 16 2.48a13.38 13.38 0 0 0-7 0C6.27.65 5.09 1 5.09 1A5.07 5.07 0 0 0 5 4.77 5.44 5.44 0 0 0 3.5 8.55c0 5.42 3.3 6.61 6.44 7A3.37 3.37 0 0 0 9 18.13V22"/>' },
  { key: 'x', label: 'X', icon: '<path d="M18.244 2.25h3.308l-7.227 8.26 8.502 11.24H16.17l-5.214-6.817L4.99 21.75H1.68l7.73-8.835L1.254 2.25H8.08l4.713 6.231z"/>' },
  { key: 'telegram', label: 'Telegram', icon: '<line x1="22" y1="2" x2="11" y2="13"/><polygon points="22 2 15 22 11 13 2 9 22 2"/>' },
]
function safeExternalUrl(value) {
  try { const url = new URL(value); return ['http:', 'https:'].includes(url.protocol) ? url.href : '' }
  catch { return '' }
}
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
    <div class="card"><h2 class="section-title">我是谁</h2><p class="about-bio">{{ about.bio || '在后台的“关于管理”中添加介绍。' }}</p><h3 v-if="about.skills?.length" class="about-subtitle">常用技能</h3><div class="skill-list"><span v-for="skill in about.skills" :key="skill" class="chip">{{ skill }}</span></div></div>
    <div class="card"><h2 class="section-title">时间线</h2><div class="timeline"><article v-for="(item, index) in about.timeline" :key="`${item.year}-${item.title}-${index}`" class="t-item"><time class="t-year">{{ item.year }}</time><h3>{{ item.title }}</h3><p>{{ item.description }}</p></article><p v-if="!about.timeline?.length" class="muted">尚未添加经历。</p></div></div>
  </div></section>
    <section class="section"><div class="container"><div class="card contact-card"><h2 class="section-title">联系我</h2><p>{{ about.contactNote || '欢迎交流与反馈，通常 24 小时内回复。' }}</p><a v-if="/^[^\s@]+@[^\s@]+\.[^\s@]+$/.test(contacts.email || '')" class="btn email-button" :href="`mailto:${contacts.email}`">{{ contacts.email }}</a><div class="social-icons"><a v-for="site in social" :key="site.key" v-show="safeExternalUrl(contacts[site.key])" :href="safeExternalUrl(contacts[site.key]) || undefined" :aria-label="site.label" :title="site.label" target="_blank" rel="noopener noreferrer"><svg :class="`social-icon-${site.key}`" viewBox="0 0 24 24" :fill="site.key === 'x' ? 'currentColor' : 'none'" aria-hidden="true" v-html="site.icon"></svg></a></div></div></div></section>
</template>
