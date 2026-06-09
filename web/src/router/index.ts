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
      name: 'DashboardV2',
      component: () => import('../components/DashboardV2.vue'),
    },
    {
      path: '/dashboard-v2',
      redirect: '/dashboard',
    },
    {
      path: '/ops-dashboard',
      name: 'OpsDashboard',
      component: () => import('../components/Dashboard.vue'),
      meta: { superAdminOnly: true },
    },
    {
      path: '/query',
      name: 'Query',
      component: () => import('../components/LogQuery.vue'),
    },
    {
      path: '/behavior',
      name: 'Behavior',
      component: () => import('../components/BehaviorTimeline.vue'),
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
      meta: { superAdminOnly: true },
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
      meta: { superAdminOnly: true },
    },
    {
      path: '/features',
      name: 'Features',
      component: () => import('../components/FeatureConfig.vue'),
      meta: { superAdminOnly: true },
    },
    {
      path: '/system',
      name: 'System',
      component: () => import('../components/SystemStatus.vue'),
      meta: { superAdminOnly: true },
    },
    {
      path: '/tasks',
      name: 'Tasks',
      component: () => import('../components/TaskCenter.vue'),
      meta: { superAdminOnly: true },
    },
    {
      path: '/users',
      name: 'Users',
      component: () => import('../components/UserManagement.vue'),
      meta: { superAdminOnly: true },
    },
  ],
})

router.beforeEach((to) => {
  const authStore = useAuthStore()
  if (!authStore.isLoggedIn) {
    if (to.path !== '/dashboard') {
      return { path: '/dashboard', query: to.fullPath !== '/dashboard' ? { redirect: to.fullPath } : undefined }
    }
    return false
  }
  if (to.meta.superAdminOnly && authStore.role !== 'super_admin') {
    return { path: '/dashboard' }
  }
})

export default router
