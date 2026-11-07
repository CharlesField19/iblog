<script setup lang="ts">
import { errorMessage } from '../../../shared/i18n'
import { onMounted, reactive, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { adminRequest } from '../api'
import type { AboutData, AdminSettings } from '../../../shared/contracts'
import { useToast, useConfirm } from '../injections'

const toast = useToast()
const { t } = useI18n()
const state = reactive<AboutData>({ intro: '', bio: '', skills: [], timeline: [], contactNote: '' })
const skill = ref('')
const loading = ref(true)
const saving = ref(false)
const error = ref('')
async function load() {
  loading.value = true
  error.value = ''
  try {
    const result = await adminRequest<AdminSettings>('/api/v1/admin/settings')
    Object.assign(state, {
      intro: '',
      bio: '',
      skills: [],
      timeline: [],
      contactNote: '',
      ...(result.settings?.about || {}),
    })
    state.skills = [...(state.skills || [])]
    state.timeline = (state.timeline || []).map((x) => ({ ...x }))
  } catch (e) {
    error.value = errorMessage(e)
  } finally {
    loading.value = false
  }
}
onMounted(load)
function addSkill() {
  const v = skill.value.trim()
  if (!v) return
  if ([...v].length > 24) {
    toast('error', t('admin.about.skillTooLong'))
    return
  }
  if (!state.skills.includes(v)) state.skills.push(v)
  skill.value = ''
}
async function save() {
  saving.value = true
  try {
    await adminRequest('/api/v1/admin/settings/about', {
      method: 'PUT',
      body: {
        ...state,
        skills: state.skills.map((x) => x.trim()).filter(Boolean),
        timeline: state.timeline.map((x) => ({
          year: x.year.trim(),
          title: x.title.trim(),
          description: x.description.trim(),
        })),
      },
    })
    toast('success', t('admin.about.saved'))
  } catch (e) {
    toast('error', errorMessage(e))
  } finally {
    saving.value = false
  }
}
</script>

<template>
  <main id="admin-content" class="admin-main" tabindex="-1">
    <div class="list-head">
      <h1>{{ t('admin.about.title') }}</h1>
      <button class="btn" :disabled="saving || loading" @click="save">
        {{ saving ? t('admin.common.saving') : t('admin.common.save') }}
      </button>
    </div>
    <p v-if="error" class="card error-card" role="alert">
      {{ error }} <button class="link-btn" @click="load">{{ t('admin.common.retry') }}</button>
    </p>
    <div v-else-if="loading" class="card skeleton">{{ t('admin.common.loading') }}</div>
    <div v-else class="stack">
      <section class="card section-card">
        <h2 class="section-title">{{ t('admin.about.intro') }}</h2>
        <div class="field">
          <label for="about-intro">{{ t('admin.about.oneLine') }}</label
          ><input
            id="about-intro"
            v-model="state.intro"
            name="intro"
            maxlength="300"
            :placeholder="t('admin.about.introPlaceholder')"
          />
        </div>
        <div class="field">
          <label for="about-bio">{{ t('admin.about.bio') }}</label
          ><textarea
            id="about-bio"
            v-model="state.bio"
            name="bio"
            rows="4"
            maxlength="5000"
            :placeholder="t('admin.about.bioPlaceholder')"
          ></textarea>
        </div>
      </section>
      <section class="card section-card">
        <h2 class="section-title">{{ t('admin.about.skills') }}</h2>
        <p class="section-hint">{{ t('admin.about.skillsHint') }}</p>
        <div class="chips">
          <span v-for="(item, index) in state.skills" :key="`${item}-${index}`" class="chip"
            >{{ item }}
            <button
              class="chip-remove"
              type="button"
              :aria-label="t('admin.about.removeSkill', { skill: item })"
              @click="state.skills.splice(index, 1)"
            >
              ×
            </button></span
          >
        </div>
        <div class="inline-form">
          <input
            v-model="skill"
            class="chip-input"
            maxlength="24"
            name="skill"
            :aria-label="t('admin.about.addSkill')"
            :placeholder="t('admin.about.skillExample')"
            autocomplete="off"
            @keydown.enter.prevent="addSkill"
          /><button class="btn btn-secondary btn-sm" type="button" @click="addSkill">
            {{ t('admin.common.add') }}
          </button>
        </div>
      </section>
      <section class="card section-card">
        <h2 class="section-title">{{ t('admin.about.timeline') }}</h2>
        <div
          v-for="(item, index) in state.timeline"
          :key="`${item.year}-${item.title}-${index}`"
          class="timeline-row"
        >
          <div class="field">
            <label :for="`timeline-year-${index}`">{{ t('admin.about.year') }}</label
            ><input
              :id="`timeline-year-${index}`"
              v-model="item.year"
              :name="`timeline-year-${index}`"
              maxlength="20"
              placeholder="2024"
            />
          </div>
          <div class="field">
            <label :for="`timeline-title-${index}`">{{ t('admin.about.titleField') }}</label
            ><input
              :id="`timeline-title-${index}`"
              v-model="item.title"
              :name="`timeline-title-${index}`"
              maxlength="200"
              :placeholder="t('admin.about.titlePlaceholder')"
            />
          </div>
          <div class="field">
            <label :for="`timeline-description-${index}`">{{ t('admin.about.description') }}</label
            ><textarea
              :id="`timeline-description-${index}`"
              v-model="item.description"
              :name="`timeline-description-${index}`"
              rows="2"
              maxlength="2000"
              :placeholder="t('admin.about.descriptionPlaceholder')"
            ></textarea>
          </div>
          <button
            class="link-btn danger"
            type="button"
            :aria-label="t('admin.about.removeTimeline', { title: item.title || index + 1 })"
            @click="state.timeline.splice(index, 1)"
          >
            {{ t('admin.about.delete') }}
          </button>
        </div>
        <button
          class="link-btn add-timeline"
          type="button"
          @click="state.timeline.push({ year: '', title: '', description: '' })"
        >
          {{ t('admin.about.addTimeline') }}
        </button>
      </section>
      <section class="card section-card">
        <h2 class="section-title">{{ t('admin.about.contactSection') }}</h2>
        <div class="field">
          <label for="contact-note">{{ t('admin.about.contactNote') }}</label
          ><input
            id="contact-note"
            v-model="state.contactNote"
            name="contactNote"
            maxlength="300"
            :placeholder="t('admin.about.contactPlaceholder')"
          />
        </div>
      </section>
    </div>
  </main>
</template>
