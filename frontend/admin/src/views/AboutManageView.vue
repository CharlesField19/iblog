<script setup>
import { inject, onMounted, reactive, ref } from 'vue'
import { adminRequest } from '../api.js'

const toast=inject('toast');const state=reactive({intro:'',bio:'',skills:[],timeline:[],contactNote:''});const skill=ref('');const loading=ref(true);const saving=ref(false);const error=ref('')
async function load(){loading.value=true;error.value='';try{const result=await adminRequest('/api/v1/admin/settings');Object.assign(state,{intro:'',bio:'',skills:[],timeline:[],contactNote:'',...(result.settings?.about||{})});state.skills=[...(state.skills||[])];state.timeline=(state.timeline||[]).map(x=>({...x}))}catch(e){error.value=e.message}finally{loading.value=false}}
onMounted(load)
function addSkill(){const v=skill.value.trim();if(!v)return;if([...v].length>24){toast('error','技能名称不能超过 24 个字符');return}if(!state.skills.includes(v))state.skills.push(v);skill.value=''}
async function save(){saving.value=true;try{await adminRequest('/api/v1/admin/settings/about',{method:'PUT',body:{...state,skills:state.skills.map(x=>x.trim()).filter(Boolean),timeline:state.timeline.map(x=>({year:x.year.trim(),title:x.title.trim(),description:x.description.trim()}))}});toast('success','关于信息已保存')}catch(e){toast('error',e.message)}finally{saving.value=false}}
</script>

<template>
  <main id="admin-content" class="admin-main" tabindex="-1"><div class="list-head"><h1>关于管理</h1><button class="btn" :disabled="saving||loading" @click="save">{{ saving?'保存中…':'保存' }}</button></div>
    <p v-if="error" class="card error-card" role="alert">{{ error }} <button class="link-btn" @click="load">重试</button></p>
    <div v-else-if="loading" class="card skeleton">加载中…</div>
    <div v-else class="stack"><section class="card section-card"><h2 class="section-title">简介</h2><div class="field"><label for="about-intro">一句话介绍</label><input id="about-intro" v-model="state.intro" name="intro" maxlength="300" placeholder="介绍一下这个博客…"></div><div class="field"><label for="about-bio">我是谁</label><textarea id="about-bio" v-model="state.bio" name="bio" rows="4" maxlength="5000" placeholder="写一段个人介绍…"></textarea></div></section>
      <section class="card section-card"><h2 class="section-title">常用技能</h2><p class="section-hint">每项最多 24 个字符；输入后回车或点击添加。</p><div class="chips"><span v-for="(item,index) in state.skills" :key="`${item}-${index}`" class="chip">{{ item }} <button class="chip-remove" type="button" :aria-label="`移除技能 ${item}`" @click="state.skills.splice(index,1)">×</button></span></div><div class="inline-form"><input v-model="skill" class="chip-input" maxlength="24" name="skill" aria-label="添加技能" placeholder="例如：Vue…" autocomplete="off" @keydown.enter.prevent="addSkill"><button class="btn btn-secondary btn-sm" type="button" @click="addSkill">添加</button></div></section>
      <section class="card section-card"><h2 class="section-title">时间线</h2><div v-for="(item,index) in state.timeline" :key="`${item.year}-${item.title}-${index}`" class="timeline-row"><div class="field"><label :for="`timeline-year-${index}`">年份</label><input :id="`timeline-year-${index}`" v-model="item.year" :name="`timeline-year-${index}`" maxlength="20" placeholder="2024"></div><div class="field"><label :for="`timeline-title-${index}`">标题</label><input :id="`timeline-title-${index}`" v-model="item.title" :name="`timeline-title-${index}`" maxlength="200" placeholder="经历标题…"></div><div class="field"><label :for="`timeline-description-${index}`">描述</label><textarea :id="`timeline-description-${index}`" v-model="item.description" :name="`timeline-description-${index}`" rows="2" maxlength="2000" placeholder="描述这段经历…"></textarea></div><button class="link-btn danger" type="button" :aria-label="`删除时间线项目 ${item.title || index + 1}`" @click="state.timeline.splice(index,1)">删除</button></div><button class="link-btn add-timeline" type="button" @click="state.timeline.push({year:'',title:'',description:''})">+ 添加一段经历</button></section>
      <section class="card section-card"><h2 class="section-title">联系卡片说明</h2><div class="field"><label for="contact-note">关于页联系卡片上的一句话</label><input id="contact-note" v-model="state.contactNote" name="contactNote" maxlength="300" placeholder="欢迎交流与反馈…"></div></section>
    </div>
  </main>
</template>
