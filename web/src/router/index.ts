import { createRouter, createWebHistory } from 'vue-router'
import { useAuthStore } from '../stores/auth'

const router = createRouter({
  history: createWebHistory(),
  routes: [
    {
      path: '/',
      redirect: '/dashboard',
    },
    {
      path: '/dashboard',
      name: 'Dashboard',
      component: () => import('../components/Dashboard.vue'),
    },
    {
      path: '/query',
      name: 'Query',
      component: () => import('../components/LogQuery.vue'),
    },
    {
      path: '/contacts',
      name: 'Contacts',
      component: () => import('../components/ContactList.vue'),
    },
    {
      path: '/sync',
      name: 'Sync',
      component: () => import('../components/DataSync.vue'),
    },
    {
      path: '/adminoper',
      name: 'AdminOper',
      component: () => import('../components/AdminOperLog.vue'),
    },
    {
      path: '/keys',
      name: 'Keys',
      component: () => import('../components/KeyManagement.vue'),
    },
    {
      path: '/features',
      name: 'Features',
      component: () => import('../components/FeatureConfig.vue'),
    },
    {
      path: '/system',
      name: 'System',
      component: () => import('../components/SystemStatus.vue'),
    },
  ],
})

router.beforeEach((to) => {
  const authStore = useAuthStore()
  if (!authStore.isLoggedIn) {
    // Not logged in — App.vue will show Login component,
    // but ensure URL is clean
    if (to.path !== '/dashboard') {
      return '/dashboard'
    }
  }
})

export default router
