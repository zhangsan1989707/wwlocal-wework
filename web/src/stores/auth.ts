import { defineStore } from 'pinia'
import { ref } from 'vue'
import { authAPI } from '../api/index'

export const useAuthStore = defineStore('auth', () => {
  const isLoggedIn = ref(authAPI.isAuthenticated())
  const username = ref(authAPI.getUsername() || '')

  function login(user: string) {
    isLoggedIn.value = true
    username.value = user
  }

  function logout() {
    authAPI.logout()
    isLoggedIn.value = false
    username.value = ''
  }

  return { isLoggedIn, username, login, logout }
})
