<script setup>
import { inject, onMounted, reactive, ref } from 'vue'
import { adminRequest } from '../api.js'

const toast=inject('toast');const items=ref([]);const loading=ref(false);const error=ref('');const editId=ref(null)
const blank=()=>({name:'',url:'',avatar:'',description:'',sortOrder:0})
const form=reactive(blank())
function reset(){Object.assign(form,blank());editId.value=null}
async function load(){loading.value=true;error.value='';try{items.value=(await adminRequest('/api/v1/admin/friends')).items||[]}catch(e){error.value=e.message}finally{loading.value=false}}
onMounted(load)
function edit(item){editId.value=item.id;Object.assign(form,{name:item.name,url:item.url,avatar:item.avatar,description:item.description,sortOrder:item.sortOrder})}
async function save(){if(!form.name.trim()||!form.url.trim()){toast('error','名称和链接不能为空');return}try{await adminRequest(editId.value?`/api/v1/admin/friends/${editId.value}`:'/api/v1/admin/friends',{method:editId.value?'PUT':'POST',body:{...form,name:form.name.trim(),url:form.url.trim()}});toast('success',editId.value?'友链已更新':'友链已添加');reset();load()}catch(e){toast('error',e.message)}}
async function remove(item){if(!confirm(`删除友链「${item.name}」？`))return;try{await adminRequest(`/api/v1/admin/friends/${item.id}`,{method:'DELETE'});toast('success','友链已删除');load()}catch(e){toast('error',e.message)}}
</script>

<template>
  <main class="admin-main"><div class="list-head"><h1>友链管理</h1></div>
    <form class="card form-card friend-form" @submit.prevent="save"><h2 class="form-title">{{ editId?'编辑友链':'添加友链' }}</h2><div class="field-row"><div class="field"><label for="friend-name">名称</label><input id="friend-name" v-model="form.name" maxlength="100" placeholder="友链名称" required></div><div class="field"><label for="friend-url">链接</label><input id="friend-url" v-model="form.url" type="url" placeholder="https://" required></div></div><div class="field-row"><div class="field"><label for="friend-avatar">头像地址</label><input id="friend-avatar" v-model="form.avatar" type="url" placeholder="https://…（可留空）"></div><div class="field"><label for="friend-sort">排序（数字越小越靠前）</label><input id="friend-sort" v-model.number="form.sortOrder" type="number"></div></div><div class="field"><label for="friend-description">简介</label><input id="friend-description" v-model="form.description" maxlength="300" placeholder="一句话介绍这个站点"></div><div class="form-actions"><button class="btn" type="submit">{{ editId?'保存修改':'添加友链' }}</button><button v-if="editId" class="btn btn-secondary" type="button" @click="reset">取消</button></div></form>
    <p v-if="error" class="card error-card" role="alert">{{ error }} <button class="link-btn" @click="load">重试</button></p><div v-else-if="loading" class="card skeleton">加载中…</div><div v-else-if="!items.length" class="card empty-state">还没有友链。</div>
    <div v-else class="stack"><section v-for="item in items" :key="item.id" class="card admin-row friend-row"><img v-if="item.avatar" class="avatar" :src="item.avatar" :alt="item.name"><span v-else class="avatar avatar-fallback">{{ item.name.slice(0,1) }}</span><div class="row-main"><h3 class="row-title">{{ item.name }}</h3><p class="row-meta">{{ item.url }} · {{ item.description }} · 排序 {{ item.sortOrder }}</p></div><div class="row-actions"><button class="link-btn" @click="edit(item)">编辑</button><button class="link-btn danger" @click="remove(item)">删除</button></div></section></div>
  </main>
</template>
