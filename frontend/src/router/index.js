import { createRouter, createWebHashHistory } from 'vue-router'
import Servers from '../views/Servers.vue'
import Subscriptions from '../views/Subscriptions.vue'
import Configuration from '../views/Configuration.vue'

const routes = [
  { path: '/', redirect: '/servers' },
  { path: '/servers', name: 'Servers', component: Servers },
  { path: '/subscriptions', name: 'Subscriptions', component: Subscriptions },
  { path: '/configuration', name: 'Configuration', component: Configuration }
]

const router = createRouter({
  history: createWebHashHistory(),
  routes
})

export default router
