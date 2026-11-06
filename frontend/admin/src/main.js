import { createApp } from 'vue'
import { createRouter, createWebHistory } from 'vue-router'
import App from './App.vue'
import { i18n } from '../../shared/i18n.js'
import LoginView from './views/LoginView.vue'
import DashboardView from './views/DashboardView.vue'
import EditorView from './views/EditorView.vue'
import LinksManageView from './views/LinksManageView.vue'
import TagManageView from './views/TagManageView.vue'
import AboutManageView from './views/AboutManageView.vue'
import SettingsView from './views/SettingsView.vue'
import AccountView from './views/AccountView.vue'
import { hasSession } from './session.js'
import './styles/original.css'
import './styles/reconstruction.css'

const router = createRouter({
  history: createWebHistory('/admin/'),
  routes: [
    { path: '/login', component: LoginView },
    { path: '/', component: DashboardView },
    { path: '/new', component: EditorView },
    { path: '/edit/:id', component: EditorView },
    { path: '/links', component: LinksManageView },
    { path: '/tags', component: TagManageView },
    { path: '/about', component: AboutManageView },
    { path: '/settings', component: SettingsView },
    { path: '/account', component: AccountView },
    { path: '/:pathMatch(.*)*', redirect: '/' },
  ],
})
router.beforeEach((to) => {
  if (to.path !== '/login' && !hasSession()) return { path: '/login', query: { next: to.fullPath } }
  if (to.path === '/login' && hasSession()) return '/'
})
createApp(App).use(router).use(i18n).mount('#app')
