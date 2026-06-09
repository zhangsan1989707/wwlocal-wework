import { defineStore } from 'pinia'
import { ref } from 'vue'
import { authAPI } from '../api/index'

export const useAuthStore = defineStore('auth', () => {
  const isLoggedIn = ref(authAPI.isAuthenticated())
  const username = ref(authAPI.getUsername() || '')
  const role = ref(authAPI.getRole() || '')

  function login(user: string, userRole?: string) {
    isLoggedIn.value = true
    username.value = user
    role.value = userRole || authAPI.getRole() || ''
  }

  function logout() {
    authAPI.logout()
    isLoggedIn.value = false
    username.value = ''
    role.value = ''
  }

  return { isLoggedIn, username, role, login, logout }
})
