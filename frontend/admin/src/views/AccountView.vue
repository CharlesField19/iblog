<script setup>
import { inject, ref } from 'vue'
import { adminRequest } from '../api.js'
import { saveSession } from '../session.js'

const toast=inject('toast');const username=ref('');const current=ref('');const next=ref('');const confirmPassword=ref('');const savingName=ref(false);const savingPassword=ref(false)
async function changeUsername(){const value=username.value.trim();if(!/^[A-Za-z0-9_-]{3,32}$/.test(value)){toast('error','用户名需为 3-32 位字母、数字、- 或 _');return}savingName.value=true;try{const res=await adminRequest('/api/v1/admin/account/username',{method:'PUT',body:{username:value}});saveSession(res);toast('success','用户名已更新');username.value=''}catch(e){toast('error',e.message)}finally{savingName.value=false}}
async function changePassword(){if(next.value.length<8){toast('error','新密码至少需要 8 位');return}if(next.value!==confirmPassword.value){toast('error','两次输入的新密码不一致');return}savingPassword.value=true;try{const res=await adminRequest('/api/v1/admin/account/password',{method:'PUT',body:{currentPassword:current.value,newPassword:next.value}});saveSession(res);toast('success','密码已修改');current.value='';next.value='';confirmPassword.value=''}catch(e){toast('error',e.message)}finally{savingPassword.value=false}}
</script>

<template>
  <main class="admin-main"><div class="list-head"><h1>账号管理</h1></div><div class="stack">
    <section class="card section-card"><h2 class="section-title">修改用户名</h2><form class="account-form" @submit.prevent="changeUsername"><div class="field"><label for="new-username">新用户名</label><input id="new-username" v-model="username" autocomplete="username" placeholder="3-32位字母数字-_" required></div><p class="section-hint">3-32 位，仅限字母、数字、- 和 _。</p><button class="btn" :disabled="savingName">{{ savingName?'保存中…':'保存用户名' }}</button></form></section>
    <section class="card section-card"><h2 class="section-title">修改密码</h2><form class="account-form" @submit.prevent="changePassword"><div class="field"><label for="current-password">当前密码</label><input id="current-password" v-model="current" type="password" autocomplete="current-password" required></div><div class="field"><label for="new-password">新密码</label><input id="new-password" v-model="next" type="password" autocomplete="new-password" minlength="8" required></div><p class="section-hint">至少 8 位。</p><div class="field"><label for="confirm-password">确认新密码</label><input id="confirm-password" v-model="confirmPassword" type="password" autocomplete="new-password" required></div><button class="btn" :disabled="savingPassword">{{ savingPassword?'保存中…':'保存密码' }}</button></form></section>
  </div></main>
</template>
