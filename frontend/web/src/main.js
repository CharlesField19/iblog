import { createApp } from 'vue'
import { createRouter, createWebHistory } from 'vue-router'
import App from './App.vue'
import { i18n } from '../../shared/i18n.js'
import HomeView from './views/HomeView.vue'
import ArticlesView from './views/ArticlesView.vue'
import PostView from './views/PostView.vue'
import AboutView from './views/AboutView.vue'
import FriendsView from './views/FriendsView.vue'
import './styles/original.css'
import './styles/reconstruction.css'

const router = createRouter({
  history: createWebHistory(),
  scrollBehavior: () => ({ top: 0 }),
  routes: [
    { path: '/', component: HomeView },
    { path: '/articles', component: ArticlesView },
    { path: '/articles/:slug', component: PostView },
    { path: '/about', component: AboutView },
    { path: '/friends', component: FriendsView },
    { path: '/:pathMatch(.*)*', redirect: '/' },
  ],
})

createApp(App).use(router).use(i18n).mount('#app')
