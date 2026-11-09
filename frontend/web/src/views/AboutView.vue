<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { getPublicData } from '../../../shared/publicData'
import type { AboutData, PublicSettings } from '../../../shared/contracts'
import { takeInitialData } from '../../../shared/initialData'
import vReveal from '../directives/reveal'

const about = ref<AboutData>({
  intro: '',
  bio: '',
  skills: [],
  timeline: [],
  contactNote: '',
})
const { t } = useI18n()
const contacts = ref<Record<string, string>>({})
const contactStatus = ref('')
const loading = ref(false)
const defaultAbout = () => ({
  intro: t('web.aboutDefaultIntro'),
  bio: t('web.aboutDefaultBio'),
  skills: [
    t('web.defaultSkillCoding'),
    t('web.defaultSkillPhotography'),
    t('web.defaultSkillWriting'),
  ],
  timeline: [
    {
      year: '2024',
      title: t('web.defaultTimelineRecentTitle'),
      description: t('web.defaultTimelineRecentDescription'),
    },
    {
      year: '2020',
      title: t('web.defaultTimelineStartTitle'),
      description: t('web.defaultTimelineStartDescription'),
    },
  ],
  contactNote: t('web.aboutDefaultContact'),
})
const defaultContacts = { email: '', github: '', x: '', telegram: '' }
const initial = takeInitialData<{
  settings?: { about?: Partial<AboutData>; contacts?: Record<string, string> }
}>('about')
const social = [
  {
    key: 'github',
    label: 'GitHub',
    icon: '<path d="M9 19c-5 1.5-5-2.5-7-3m14 6v-3.87a3.37 3.37 0 0 0-.94-2.61c3.14-.35 6.44-1.54 6.44-7A5.44 5.44 0 0 0 20 4.77 5.07 5.07 0 0 0 19.91 1S18.73.65 16 2.48a13.38 13.38 0 0 0-7 0C6.27.65 5.09 1 5.09 1A5.07 5.07 0 0 0 5 4.77 5.44 5.44 0 0 0 3.5 8.55c0 5.42 3.3 6.61 6.44 7A3.37 3.37 0 0 0 9 18.13V22"/>',
  },
  {
    key: 'x',
    label: 'X',
    icon: '<path d="M18.244 2.25h3.308l-7.227 8.26 8.502 11.24H16.17l-5.214-6.817L4.99 21.75H1.68l7.73-8.835L1.254 2.25H8.08l4.713 6.231z"/>',
  },
  {
    key: 'telegram',
    label: 'Telegram',
    icon: '<line x1="22" y1="2" x2="11" y2="13"/><polygon points="22 2 15 22 11 13 2 9 22 2"/>',
  },
]
function safeExternalUrl(value: string, key: string) {
  const text = String(value || '').trim()
  if (!text) return ''
  const handle = text.replace(/^@/, '')
  const input = /^[a-z][a-z\d+.-]*:/i.test(text)
    ? text
    : text.startsWith('@') && key === 'telegram'
      ? `https://t.me/${handle}`
      : text.startsWith('@') && key === 'x'
        ? `https://x.com/${handle}`
        : `https://${text}`
  try {
    const url = new URL(input)
    return ['http:', 'https:'].includes(url.protocol) ? url.href : ''
  } catch {
    return ''
  }
}
async function copyEmail() {
  const email = String(contacts.value.email || '').trim()
  if (!email) return
  try {
    await navigator.clipboard.writeText(email)
    contactStatus.value = t('web.copied')
  } catch {
    contactStatus.value = t('web.copyFailed')
  }
  window.setTimeout(() => {
    contactStatus.value = ''
  }, 2200)
}
onMounted(async () => {
  about.value = { ...defaultAbout(), ...(initial?.settings?.about || {}) }
  contacts.value = { ...defaultContacts, ...(initial?.settings?.contacts || {}) }
  if (initial) return
  try {
    const settings = await getPublicData<PublicSettings>('/api/v1/settings')
    about.value = { ...about.value, ...(settings.about || {}) }
    contacts.value = { ...defaultContacts, ...(settings.contacts || {}) }
  } catch {
    /* leave the page usable if settings are unavailable */
  } finally {
    loading.value = false
  }
})
</script>

<template>
  <section class="page-head">
    <div class="page-container">
      <h1>{{ t('web.about') }}</h1>
      <p class="sub">{{ about.intro || t('web.aboutDefaultIntro') }}</p>
    </div>
  </section>
  <section class="section" v-reveal>
    <div class="page-container about-grid">
      <div class="card" v-reveal>
        <h2 class="section-title">{{ t('web.whoAmI') }}</h2>
        <p class="about-bio">{{ about.bio || t('web.aboutDefaultBio') }}</p>
        <h3 v-if="about.skills?.length" class="about-subtitle">{{ t('web.skills') }}</h3>
        <div class="skill-list">
          <span v-for="skill in about.skills" :key="skill" class="chip">{{ skill }}</span>
        </div>
      </div>
      <div class="card" v-reveal>
        <h2 class="section-title">{{ t('web.timeline') }}</h2>
        <div class="timeline">
          <article
            v-for="(item, index) in about.timeline"
            :key="`${item.year}-${item.title}-${index}`"
            class="t-item"
          >
            <time class="t-year">{{ item.year }}</time>
            <h3>{{ item.title }}</h3>
            <p>{{ item.description }}</p>
          </article>
          <p v-if="!about.timeline?.length" class="muted">{{ t('web.noTimeline') }}</p>
        </div>
      </div>
    </div>
  </section>
  <section class="section" v-reveal>
    <div class="page-container">
      <div class="card contact-card" v-reveal>
        <h2 class="section-title">{{ t('web.contact') }}</h2>
        <p>{{ about.contactNote || t('web.aboutDefaultContact') }}</p>
        <button
          v-if="contacts.email?.trim()"
          class="btn email-button"
          type="button"
          @click="copyEmail"
        >
          {{ contacts.email.trim() }}</button
        ><span v-if="contactStatus" class="contact-status" role="status" aria-live="polite">{{
          contactStatus
        }}</span>
        <div
          class="social-icons"
          v-if="social.some((site) => safeExternalUrl(contacts[site.key], site.key))"
        >
          <a
            v-for="site in social"
            :key="site.key"
            v-show="safeExternalUrl(contacts[site.key], site.key)"
            class="social-contact"
            :href="safeExternalUrl(contacts[site.key], site.key) || undefined"
            :aria-label="site.label"
            :title="site.label"
            target="_blank"
            rel="noopener noreferrer"
            ><svg
              :class="`social-icon-${site.key}`"
              viewBox="0 0 24 24"
              :fill="site.key === 'x' ? 'currentColor' : 'none'"
              aria-hidden="true"
              v-html="site.icon"
            ></svg
          ></a>
        </div>
      </div>
    </div>
  </section>
</template>
