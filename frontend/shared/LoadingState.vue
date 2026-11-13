<script setup lang="ts">
import { onBeforeUnmount, onMounted, ref } from 'vue'

const props = withDefaults(
  defineProps<{
    label?: string
    variant?: 'page' | 'card' | 'inline'
    /** Delay before showing, so fast responses do not flash a spinner. */
    delay?: number
  }>(),
  { label: '', variant: 'page', delay: 200 },
)

const visible = ref(props.variant === 'inline' || props.delay <= 0)
let timer: number | undefined

onMounted(() => {
  if (visible.value) return
  timer = window.setTimeout(() => {
    visible.value = true
  }, props.delay)
})

onBeforeUnmount(() => {
  if (timer !== undefined) window.clearTimeout(timer)
})
</script>

<template>
  <div
    v-if="visible"
    class="loading-state"
    :class="`loading-state--${variant}`"
    :role="variant === 'inline' ? undefined : 'progressbar'"
    :aria-label="variant === 'inline' ? undefined : label || undefined"
    :aria-hidden="variant === 'inline' ? 'true' : undefined"
  >
    <span class="loading-ring" aria-hidden="true"></span>
    <span v-if="label" class="loading-label">{{ label }}</span>
  </div>
</template>

<style scoped>
.loading-state {
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 10px;
  color: var(--text-3, #86868b);
}
.loading-state--page {
  flex-direction: column;
  min-height: min(50svh, 420px);
}
.loading-state--card {
  flex-direction: column;
  min-height: 180px;
  padding: 24px;
}
.loading-state--inline {
  gap: 8px;
}
.loading-ring {
  display: block;
  width: 24px;
  height: 24px;
  border: 2px solid var(--border-strong, rgba(128, 128, 128, 0.32));
  border-top-color: var(--accent, #0071e3);
  border-radius: 50%;
  animation: loading-spin 0.9s linear infinite;
}
.loading-state--inline .loading-ring {
  width: 13px;
  height: 13px;
  border-width: 1.5px;
}
.loading-label {
  font-size: 12px;
  letter-spacing: 0.01em;
}
@keyframes loading-spin {
  to {
    transform: rotate(360deg);
  }
}
@media (prefers-reduced-motion: reduce) {
  .loading-ring {
    animation: none;
  }
}
</style>
