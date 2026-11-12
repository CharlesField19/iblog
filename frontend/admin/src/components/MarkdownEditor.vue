<script setup lang="ts">
import { watch, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { EditorContent, useEditor } from '@tiptap/vue-3'
import StarterKit from '@tiptap/starter-kit'
import { Markdown } from '@tiptap/markdown'
import { adminRequest } from '../api'

const props = defineProps<{ modelValue: string }>()
const emit = defineEmits<{ 'update:modelValue': [value: string] }>()
const { t } = useI18n()
const markdownMode = ref(false)
const previewOpen = ref(false)
const previewLoading = ref(false)
const previewHtml = ref('')
const previewFailed = ref(false)

const editor = useEditor({
  extensions: [StarterKit, Markdown],
  content: props.modelValue,
  contentType: 'markdown',
  onUpdate: ({ editor }) => emit('update:modelValue', editor.getMarkdown()),
})

watch(
  () => props.modelValue,
  (value) => {
    if (!editor.value || editor.value.getMarkdown() === value) return
    editor.value.commands.setContent(value, { contentType: 'markdown', emitUpdate: false })
  },
)

function setBlock(value: string) {
  const instance = editor.value
  if (!instance) return
  if (value === 'paragraph') instance.chain().focus().setParagraph().run()
  else
    instance
      .chain()
      .focus()
      .toggleHeading({ level: Number(value) as 1 | 2 | 3 })
      .run()
}

function addLink() {
  const instance = editor.value
  if (!instance) return
  const previous = instance.getAttributes('link').href as string | undefined
  const url = window.prompt('输入链接地址', previous || 'https://')
  if (url === null) return
  if (!url.trim()) instance.chain().focus().unsetLink().run()
  else instance.chain().focus().setLink({ href: url.trim() }).run()
}

async function openMarkdownPreview() {
  previewOpen.value = true
  previewLoading.value = true
  previewFailed.value = false
  previewHtml.value = ''
  try {
    const result = await adminRequest<{ html: string }>('/api/v1/admin/markdown/preview', {
      method: 'POST',
      body: { contentMd: props.modelValue },
    })
    previewHtml.value = result.html
  } catch {
    previewFailed.value = true
  } finally {
    previewLoading.value = false
  }
}
</script>

<template>
  <div class="word-editor">
    <div class="word-toolbar" role="toolbar" aria-label="正文格式工具栏">
      <label v-if="!markdownMode" class="toolbar-select-label">
        <span class="sr-only">段落样式</span>
        <select
          :value="
            editor?.isActive('heading', { level: 1 })
              ? '1'
              : editor?.isActive('heading', { level: 2 })
                ? '2'
                : editor?.isActive('heading', { level: 3 })
                  ? '3'
                  : 'paragraph'
          "
          aria-label="段落样式"
          @change="setBlock(($event.target as HTMLSelectElement).value)"
        >
          <option value="paragraph">正文</option>
          <option value="1">标题 1</option>
          <option value="2">标题 2</option>
          <option value="3">标题 3</option>
        </select>
      </label>
      <span v-if="!markdownMode" class="toolbar-divider" aria-hidden="true"></span>
      <button
        v-if="!markdownMode"
        type="button"
        class="toolbar-button"
        :class="{ active: editor?.isActive('bold') }"
        aria-label="加粗"
        title="加粗"
        :aria-pressed="editor?.isActive('bold') || false"
        @click="editor?.chain().focus().toggleBold().run()"
      >
        <strong>B</strong>
      </button>
      <button
        v-if="!markdownMode"
        type="button"
        class="toolbar-button"
        :class="{ active: editor?.isActive('italic') }"
        aria-label="斜体"
        title="斜体"
        :aria-pressed="editor?.isActive('italic') || false"
        @click="editor?.chain().focus().toggleItalic().run()"
      >
        <em>I</em>
      </button>
      <button
        v-if="!markdownMode"
        type="button"
        class="toolbar-button"
        :class="{ active: editor?.isActive('strike') }"
        aria-label="删除线"
        title="删除线"
        :aria-pressed="editor?.isActive('strike') || false"
        @click="editor?.chain().focus().toggleStrike().run()"
      >
        <s>S</s>
      </button>
      <button
        v-if="!markdownMode"
        type="button"
        class="toolbar-button"
        :class="{ active: editor?.isActive('bulletList') }"
        aria-label="项目符号列表"
        title="项目符号列表"
        :aria-pressed="editor?.isActive('bulletList') || false"
        @click="editor?.chain().focus().toggleBulletList().run()"
      >
        • 列表
      </button>
      <button
        v-if="!markdownMode"
        type="button"
        class="toolbar-button"
        :class="{ active: editor?.isActive('orderedList') }"
        aria-label="编号列表"
        title="编号列表"
        :aria-pressed="editor?.isActive('orderedList') || false"
        @click="editor?.chain().focus().toggleOrderedList().run()"
      >
        1. 列表
      </button>
      <button
        v-if="!markdownMode"
        type="button"
        class="toolbar-button"
        :class="{ active: editor?.isActive('blockquote') }"
        aria-label="引用"
        title="引用"
        :aria-pressed="editor?.isActive('blockquote') || false"
        @click="editor?.chain().focus().toggleBlockquote().run()"
      >
        引用
      </button>
      <button
        v-if="!markdownMode"
        type="button"
        class="toolbar-button"
        :class="{ active: editor?.isActive('codeBlock') }"
        aria-label="代码块"
        title="代码块"
        :aria-pressed="editor?.isActive('codeBlock') || false"
        @click="editor?.chain().focus().toggleCodeBlock().run()"
      >
        代码
      </button>
      <button
        v-if="!markdownMode"
        type="button"
        class="toolbar-button"
        aria-label="插入链接"
        title="插入链接"
        @click="addLink"
      >
        链接
      </button>
      <span class="toolbar-spacer"></span>
      <button
        v-if="markdownMode"
        type="button"
        class="toolbar-button"
        :disabled="previewLoading"
        @click="openMarkdownPreview"
      >
        {{ t('admin.editor.markdownPreview') }}
      </button>
      <button
        type="button"
        class="mode-button"
        :aria-pressed="markdownMode"
        @click="markdownMode = !markdownMode"
      >
        {{ markdownMode ? '可视化编辑' : 'Markdown' }}
      </button>
    </div>
    <textarea
      v-if="markdownMode"
      class="markdown-source"
      :value="modelValue"
      aria-label="Markdown 正文源码"
      spellcheck="false"
      @input="emit('update:modelValue', ($event.target as HTMLTextAreaElement).value)"
    ></textarea>
    <EditorContent v-else :editor="editor" class="word-editor-content" />
  </div>
  <div
    v-if="previewOpen"
    class="friend-modal-backdrop"
    role="presentation"
    @click.self="previewOpen = false"
  >
      <section
        class="card markdown-preview-modal"
        role="dialog"
        aria-modal="true"
        aria-labelledby="markdown-preview-title"
      >
        <div class="friend-modal-head">
          <h2 id="markdown-preview-title" class="form-title">
            {{ t('admin.editor.markdownPreviewTitle') }}
          </h2>
          <button
            class="friend-modal-close"
            type="button"
            :aria-label="t('admin.editor.closePreview')"
            @click="previewOpen = false"
          >
            ×
          </button>
        </div>
        <p v-if="previewLoading" class="markdown-preview-status" role="status">
          {{ t('admin.editor.previewLoading') }}
        </p>
        <p v-else-if="previewFailed" class="markdown-preview-status" role="alert">
          {{ t('admin.editor.previewFailed') }}
        </p>
        <div v-else class="preview-scroll">
          <div class="md-preview markdown-body" v-html="previewHtml"></div>
        </div>
      </section>
  </div>
</template>
