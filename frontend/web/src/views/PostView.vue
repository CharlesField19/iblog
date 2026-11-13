<script setup lang="ts">
import { errorMessage } from '../../../shared/i18n'
import { useI18n } from 'vue-i18n'
import { computed, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { request } from '../../../shared/api'
import type { ArticleDetail } from '../../../shared/contracts'
import { takeInitialData } from '../../../shared/initialData'
import LoadingState from '../../../shared/LoadingState.vue'
import vReveal from '../directives/reveal'

const route = useRoute()
const router = useRouter()
const { t, locale } = useI18n()
const post = ref<ArticleDetail | null>(null)
const error = ref('')
const loading = ref(true)
const showTop = ref(false)
const shareState = ref('')
function setMeta(name: string, content: string, property = false) {
  const selector = property ? `meta[property="${name}"]` : `meta[name="${name}"]`
  let element = document.head.querySelector(selector)
  if (!element) {
    element = document.createElement('meta')
    element.setAttribute(property ? 'property' : 'name', name)
    document.head.appendChild(element)
  }
  element.setAttribute('content', content || '')
}
function updateArticleMeta(article: ArticleDetail | null) {
  const title = article ? `${article.title} · iBlog` : `${t('web.articles')} · iBlog`
  const description = article?.summary || t('web.articlesIntro')
  const canonical = new URL(
    article ? `/articles/${encodeURIComponent(article.slug)}` : '/articles',
    window.location.origin,
  ).href
  const image = new URL('/favicon.ico', window.location.origin).href
  document.title = title
  setMeta('description', description)
  let link = document.head.querySelector<HTMLLinkElement>('link[rel="canonical"]')
  if (!link) {
    link = document.createElement('link')
    link.rel = 'canonical'
    document.head.appendChild(link)
  }
  link.href = canonical
  setMeta('robots', 'index,follow')
  setMeta('og:title', title, true)
  setMeta('og:description', description, true)
  setMeta('og:url', canonical, true)
  setMeta('og:type', article ? 'article' : 'website', true)
  setMeta('og:image', image, true)
  setMeta('twitter:card', 'summary')
  setMeta('twitter:title', title)
  setMeta('twitter:description', description)
  setMeta('twitter:image', image)
}
function headingId(text: string, index: number) {
  const base =
    text
      .toLowerCase()
      .replace(/<[^>]+>/g, '')
      .replace(/[^\x00-\x7f\w]+/g, '-')
      .replace(/^-+|-+$/g, '') || `section-${index + 1}`
  return `heading-${index + 1}-${base}`
}
const rendered = computed(() => {
  const template = document.createElement('template')
  template.innerHTML = post.value?.contentHtml || ''
  const toc = [...template.content.querySelectorAll('h2,h3,h4')].map((heading, index) => {
    const text = heading.textContent || ''
    const id = headingId(text, index)
    heading.id = id
    return { text, id, level: Number(heading.tagName.slice(1)) }
  })
  return { html: template.innerHTML, toc, text: template.content.textContent || '' }
})
const toc = computed(() => rendered.value.toc)
const html = computed(() => rendered.value.html)
const readingTime = computed(() =>
  Math.max(1, Math.ceil(rendered.value.text.replace(/\s/g, '').length / 450)),
)
function formatDate(value?: string | null) {
  return value
    ? new Intl.DateTimeFormat(locale.value, {
        year: 'numeric',
        month: 'long',
        day: 'numeric',
      }).format(new Date(value))
    : ''
}
let loadVersion = 0
let loadController: AbortController | undefined
let viewSlug = ''
async function load(slug: string | string[]) {
  slug = Array.isArray(slug) ? slug[0] : slug
  if (post.value?.slug === slug) return
  const version = ++loadVersion
  loadController?.abort()
  const controller = new AbortController()
  loadController = controller
  loading.value = true
  error.value = ''
  post.value = null
  try {
    const initial = takeInitialData<{ article?: ArticleDetail }>('article')
    const initialArticle = initial?.article
    const article =
      initialArticle?.slug === slug
        ? initialArticle
        : await request<ArticleDetail>(`/api/v1/articles/${encodeURIComponent(slug)}`, {
            signal: controller.signal,
          })
    if (version !== loadVersion) return
    post.value = article
    updateArticleMeta(post.value)
    if (viewSlug !== article.slug) {
      viewSlug = article.slug
      void request(`/api/v1/articles/${encodeURIComponent(article.slug)}/view`, {
        method: 'POST',
      }).catch(() => {})
    }
    if (post.value.slug !== slug) router.replace(`/articles/${encodeURIComponent(post.value.slug)}`)
  } catch (e) {
    if (version !== loadVersion || controller.signal.aborted) return
    error.value = errorMessage(e)
  } finally {
    if (version === loadVersion) loading.value = false
  }
}
watch(() => route.params.slug, load, { immediate: true })
function scrollHandler() {
  showTop.value = window.scrollY > 420
}
function scrollTop() {
  const reduce = window.matchMedia('(prefers-reduced-motion: reduce)').matches
  window.scrollTo({ top: 0, behavior: reduce ? 'auto' : 'smooth' })
}
async function shareArticle() {
  if (!post.value) return
  const url = window.location.href
  try {
    if (navigator.share) {
      await navigator.share({
        title: post.value.title,
        text: post.value.summary || post.value.title,
        url,
      })
      shareState.value = t('web.shareOpened')
      return
    }
  } catch {}
  try {
    await navigator.clipboard.writeText(url)
    shareState.value = t('web.copiedLink')
  } catch {
    shareState.value = t('web.shareFailed')
  }
  setTimeout(() => {
    shareState.value = ''
  }, 2200)
}
async function copyLink() {
  try {
    await navigator.clipboard.writeText(window.location.href)
    shareState.value = t('web.copiedLink')
  } catch {
    shareState.value = t('web.copyLinkFailed')
  }
  setTimeout(() => {
    shareState.value = ''
  }, 2200)
}
onMounted(() => window.addEventListener('scroll', scrollHandler, { passive: true }))
onBeforeUnmount(() => {
  window.removeEventListener('scroll', scrollHandler)
  ++loadVersion
  loadController?.abort()
})
</script>

<template>
  <section class="section post-page" v-reveal>
    <div class="page-container">
      <RouterLink class="back-link btn" to="/articles">{{ t('web.back') }}</RouterLink>
      <LoadingState v-if="loading" variant="page" :label="t('web.loadingArticles')" />
      <div v-else-if="error" role="alert">
        <p>{{ error }}</p>
        <RouterLink to="/articles">{{ t('web.seeOtherArticles') }}</RouterLink>
      </div>
      <article v-else-if="post">
        <h1 class="post-title">{{ post.title }}</h1>
        <div class="post-meta">
          <RouterLink
            v-for="tag in post.tags"
            :key="tag"
            class="tag"
            :to="`/articles?tag=${encodeURIComponent(tag)}`"
            >{{ tag }}</RouterLink
          ><time :datetime="post.publishedAt || post.updatedAt">{{
            formatDate(post.publishedAt || post.updatedAt)
          }}</time
          ><span class="post-views">{{
            t('web.views', { count: new Intl.NumberFormat(locale).format(post.views || 0) })
          }}</span
          ><span>{{ t('web.readingMinutes', { count: readingTime }) }}</span>
        </div>
        <p class="post-summary">{{ post.summary }}</p>
        <div class="post-share" :aria-label="t('web.share')">
          <button class="btn btn-secondary" type="button" @click="copyLink">
            {{ t('web.copyLink') }}</button
          ><button class="btn btn-secondary" type="button" @click="shareArticle">
            {{ t('web.share') }}</button
          ><span v-if="shareState" class="share-status" role="status">{{ shareState }}</span>
        </div>
        <nav v-if="toc.length" class="post-toc" :aria-label="t('web.articleToc')">
          <strong>{{ t('web.toc') }}</strong
          ><a
            v-for="item in toc"
            :key="item.id"
            :class="`toc-level-${item.level}`"
            :href="`#${item.id}`"
            >{{ item.text }}</a
          >
        </nav>
        <div class="markdown-body" v-html="html"></div>
        <nav class="post-neighbors" :aria-label="t('web.neighboringArticles')">
          <RouterLink
            v-if="post.previous"
            :to="`/articles/${encodeURIComponent(post.previous.slug)}`"
            >{{ t('web.previous') }}: {{ post.previous.title }}</RouterLink
          ><RouterLink v-if="post.next" :to="`/articles/${encodeURIComponent(post.next.slug)}`"
            >{{ t('web.next') }}: {{ post.next.title }}</RouterLink
          >
        </nav>
      </article>
    </div>
  </section>
  <button
    v-if="showTop"
    class="back-to-top"
    type="button"
    :aria-label="t('web.backToTop')"
    @click="scrollTop"
  >
    <svg
      viewBox="0 0 24 24"
      fill="none"
      stroke="currentColor"
      stroke-width="1.8"
      stroke-linecap="round"
      stroke-linejoin="round"
      aria-hidden="true"
    >
      <path d="M12 19V5" />
      <path d="m5 12 7-7 7 7" />
      <path d="M5 19h14" />
    </svg>
  </button>
</template>
