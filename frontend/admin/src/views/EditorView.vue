<script setup>
import { computed, inject, onMounted, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { marked } from 'marked'
import DOMPurify from 'dompurify'
import { adminRequest } from '../api.js'

const route=useRoute();const router=useRouter();const toast=inject('toast');const id=route.params.id
const title=ref('');const slug=ref('');const summary=ref('');const content=ref('');const status=ref('draft');const tags=ref([]);const tagInput=ref('');const knownTags=ref([]);const loading=ref(Boolean(id));const saving=ref(false);const error=ref('')
const preview=computed(()=>DOMPurify.sanitize(marked.parse(content.value||'')))
const suggestions=computed(()=>knownTags.value.filter(x=>!tags.value.some(t=>t.toLowerCase()===x.toLowerCase())&&(!tagInput.value||x.toLowerCase().includes(tagInput.value.toLowerCase()))).slice(0,8))
function addTag(value=tagInput.value){const v=value.trim();if(!v)return;if(tags.value.length>=10){toast('error','标签数量上限为 10 个');return}if([...v].length>24){toast('error','标签最长 24 个字符');return}if(tags.value.some(x=>x.toLowerCase()===v.toLowerCase())){toast('error','标签已存在');return}tags.value.push(v);tagInput.value=''}
async function init(){try{knownTags.value=(await adminRequest('/api/v1/admin/tags')).items.map(x=>x.name)}catch{}if(id){try{const a=await adminRequest(`/api/v1/admin/articles/${id}`);title.value=a.title;slug.value=a.slug;summary.value=a.summary;content.value=a.contentMd||'';status.value=a.status;tags.value=[...(a.tags||[])]}catch(e){error.value=e.message}}loading.value=false}
onMounted(init)
async function save(){if(!title.value.trim()){toast('error','请填写标题');return}if([...title.value.trim()].length>200){toast('error','标题不能超过 200 个字符');return}if([...summary.value].length>1000){toast('error','摘要不能超过 1000 个字符');return}if([...content.value].length>200000){toast('error','正文不能超过 200000 个字符');return}if(id&&!slug.value.trim()){toast('error','更新时必须填写 slug');return}if([...slug.value].length>200){toast('error','Slug 不能超过 200 个字符');return}saving.value=true;try{const body={title:title.value.trim(),summary:summary.value.trim(),contentMd:content.value,status:status.value,tags:tags.value,...(slug.value.trim()?{slug:slug.value.trim()}:{})};if(id)await adminRequest(`/api/v1/admin/articles/${id}`,{method:'PUT',body});else await adminRequest('/api/v1/admin/articles',{method:'POST',body});toast('success',id?'文章已更新':'文章已创建');router.push('/')}catch(e){toast('error',e.message)}finally{saving.value=false}}
</script>

<template>
  <main id="admin-content" class="admin-main" tabindex="-1">
    <div class="list-head"><h1>{{ id?'编辑文章':'新建文章' }}</h1><div class="head-actions"><RouterLink class="btn btn-secondary" to="/">返回</RouterLink><button class="btn" :disabled="saving||loading" @click="save">{{ saving?'保存中…':'保存' }}</button></div></div>
    <div v-if="loading" class="card skeleton sk-line">加载中…</div><div v-else-if="error" class="card error-card" role="alert">{{ error }}</div>
    <form v-else class="editor-form" @submit.prevent="save">
      <section class="card form-card">
        <div class="field"><label for="article-title">标题 <span class="field-counter">{{ [...title].length }} / 200</span></label><input id="article-title" v-model="title" name="title" placeholder="文章标题" maxlength="200" required></div>
        <div class="field"><label for="article-slug">Slug <span class="field-hint-inline">新建时留空将自动生成</span></label><input id="article-slug" v-model="slug" name="slug" placeholder="例如：my-first-post" maxlength="200" autocomplete="off"></div>
        <div class="field"><label for="article-tags">标签 <span class="field-hint-inline">最多 10 个，每个不超过 24 字</span></label><div class="chips"><span v-for="(tag,index) in tags" :key="tag" class="chip">{{ tag }} <button type="button" class="chip-remove" :aria-label="`移除 ${tag}`" @click="tags.splice(index,1)">×</button></span></div><div class="chip-editor"><input id="article-tags" v-model="tagInput" class="chip-input" name="tags" placeholder="输入标签后回车或点击添加…" autocomplete="off" @keydown.enter.prevent="addTag"><button class="btn btn-secondary btn-sm" type="button" @click="addTag()">添加标签</button></div><div v-if="suggestions.length" class="tag-suggestions"><button v-for="tag in suggestions" :key="tag" class="tag-suggestion" type="button" @click="addTag(tag)">{{ tag }}</button></div></div>
        <div class="field"><label for="article-summary">摘要 <span class="field-counter">{{ [...summary].length }} / 1000</span></label><textarea id="article-summary" v-model="summary" name="summary" rows="3" maxlength="1000" placeholder="显示在文章列表中的简短介绍…"></textarea></div>
        <fieldset class="field status-field"><legend>状态</legend><div class="radio-pills"><label><input v-model="status" type="radio" name="status" value="draft"><span>草稿</span></label><label><input v-model="status" type="radio" name="status" value="published"><span>已发布</span></label></div></fieldset>
      </section>
      <section class="editor-grid"><div class="field editor-pane"><label for="article-content">正文（Markdown） <span class="field-counter">{{ [...content].length.toLocaleString('zh-CN') }} / 200,000</span></label><textarea id="article-content" v-model="content" name="contentMd" class="markdown-input" placeholder="# 标题&#10;&#10;在这里开始写作，支持 Markdown 语法…" required></textarea></div><div class="editor-pane"><label for="article-content">实时预览</label><div class="md-preview card preview-scroll" v-html="preview"></div></div></section>
    </form>
  </main>
</template>
