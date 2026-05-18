import { createRouter, createWebHistory } from 'vue-router'
import { authAPI } from '../api/index'

const router = createRouter({
  history: createWebHistory(),
  routes: [
    {
      path: '/login',
      name: 'Login',
      component: () => import('../components/Login.vue'),
    },
    {
      path: '/',
      redirect: '/dashboard',
    },
    {
      path: '/dashboard',
      name: 'Dashboard',
      component: () => import('../components/Dashboard.vue'),
      meta: { requiresAuth: true },
    },
    {
      path: '/query',
      name: 'Query',
      component: () => import('../components/LogQuery.vue'),
      meta: { requiresAuth: true },
    },
    {
      path: '/contacts',
      name: 'Contacts',
      component: () => import('../components/ContactList.vue'),
      meta: { requiresAuth: true },
    },
    {
      path: '/sync',
      name: 'Sync',
      component: () => import('../components/DataSync.vue'),
      meta: { requiresAuth: true },
    },
    {
      path: '/adminoper',
      name: 'AdminOper',
      component: () => import('../components/AdminOperLog.vue'),
      meta: { requiresAuth: true },
    },
    {
      path: '/keys',
      name: 'Keys',
      component: () => import('../components/KeyManagement.vue'),
      meta: { requiresAuth: true },
    },
    {
      path: '/features',
      name: 'Features',
      component: () => import('../components/FeatureConfig.vue'),
      meta: { requiresAuth: true },
    },
    {
      path: '/system',
      name: 'System',
      component: () => import('../components/SystemStatus.vue'),
      meta: { requiresAuth: true },
    },
  ],
})

router.beforeEach((to) => {
  if (to.meta.requiresAuth && !authAPI.isAuthenticated()) {
    return { name: 'Login' }
  }
  if (to.name === 'Login' && authAPI.isAuthenticated()) {
    return { name: 'Dashboard' }
  }
})

export default router
