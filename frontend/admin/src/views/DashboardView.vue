<script setup>
import { computed, inject, onMounted, ref, watch } from 'vue'
import { RouterLink } from 'vue-router'
import { adminRequest } from '../api.js'

const toast=inject('toast');const items=ref([]);const total=ref(0);const page=ref(1);const status=ref('');const search=ref('');const query=ref('');const loading=ref(false);const error=ref('');const pageSize=10
const pages=computed(()=>Math.max(1,Math.ceil(total.value/pageSize)))
let timer
async function load(){loading.value=true;error.value='';const p=new URLSearchParams({page:String(page.value),pageSize:String(pageSize)});if(status.value)p.set('status',status.value);if(query.value)p.set('q',query.value);try{const d=await adminRequest(`/api/v1/admin/articles?${p}`);items.value=d.items||[];total.value=d.total||0}catch(e){error.value=e.message}finally{loading.value=false}}
watch([status,page],load);watch(search,()=>{clearTimeout(timer);timer=setTimeout(()=>{query.value=search.value.trim();page.value=1;load()},250)})
onMounted(load)
async function remove(post){if(!confirm(`删除文章「${post.title}」？`))return;try{await adminRequest(`/api/v1/admin/articles/${post.id}`,{method:'DELETE'});toast('success','文章已删除');if(items.value.length===1&&page.value>1)page.value--;else load()}catch(e){toast('error',e.message)}}
function formatDate(s){return s?new Date(s).toLocaleString('zh-CN',{hour12:false}):'—'}
</script>

<template>
  <main class="admin-main">
    <div class="list-head"><h1>文章管理 <span class="count">{{ total }}</span></h1><RouterLink class="btn" to="/new">新建文章</RouterLink></div>
    <div class="toolbar"><div class="pills"><button v-for="item in [{value:'',label:'全部'},{value:'draft',label:'草稿'},{value:'published',label:'已发布'}]" :key="item.value" class="pill" :class="{active:status===item.value}" @click="status=item.value;page=1">{{ item.label }}</button></div><label class="search-box"><span aria-hidden="true">⌕</span><input v-model="search" type="search" placeholder="搜索标题或标签…" aria-label="搜索文章"></label></div>
    <p v-if="error" class="card error-card" role="alert">{{ error }} <button class="link-btn" @click="load">重试</button></p>
    <div v-else-if="loading" class="stack"><div v-for="n in 3" :key="n" class="card skeleton sk-line"></div></div>
    <div v-else-if="!items.length" class="card empty-state">{{ query || status ? '没有符合条件的文章。' : '还没有文章，点击“新建文章”开始写作。' }}</div>
    <div v-else class="stack"><article v-for="post in items" :key="post.id" class="card admin-row"><div class="row-main"><div class="row-title"><h3>{{ post.title }}</h3><span class="status-pill" :class="post.status">{{ post.status==='published'?'已发布':'草稿' }}</span></div><p class="row-meta"><span class="slug">{{ post.slug }}</span><span>{{ formatDate(post.updatedAt) }}</span><span>{{ post.views }} 次浏览</span></p><div class="tag-list"><span v-for="tag in post.tags" :key="tag" class="tag">{{ tag }}</span></div></div><div class="row-actions"><RouterLink class="link-btn" :to="`/edit/${post.id}`">编辑</RouterLink><button class="link-btn danger" @click="remove(post)">删除</button></div></article></div>
    <div v-if="!loading && total" class="pager"><button class="btn btn-secondary btn-sm" :disabled="page<=1" @click="page--">上一页</button><span class="pager-info">第 {{ page }} / {{ pages }} 页 · 共 {{ total }} 篇</span><button class="btn btn-secondary btn-sm" :disabled="page>=pages" @click="page++">下一页</button></div>
  </main>
</template>
