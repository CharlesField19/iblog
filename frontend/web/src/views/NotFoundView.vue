<script setup lang="ts">
import { useI18n } from 'vue-i18n'

const { t } = useI18n()
</script>

<template>
  <section class="page-head">
    <div class="page-container">
      <h1>{{ t('web.notFoundTitle') }}</h1>
      <p class="sub">{{ t('web.notFoundDescription') }}</p>
      <RouterLink class="btn" to="/">{{ t('web.returnHome') }}</RouterLink>
    </div>
  </section>
</template>
