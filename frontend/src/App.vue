<template>
  <div class="min-h-screen">
    <!-- Sidebar + Main -->
    <div v-if="auth.isAuthenticated" class="flex h-screen">
      <!-- Sidebar -->
      <aside class="w-64 bg-dark-900 border-r border-dark-700 flex flex-col">
        <div class="p-6">
          <h1 class="text-xl font-bold text-primary-400 flex items-center gap-2">
            <svg class="w-7 h-7" fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M5 12h14M5 12a2 2 0 01-2-2V6a2 2 0 012-2h14a2 2 0 012 2v4a2 2 0 01-2 2M5 12a2 2 0 00-2 2v4a2 2 0 002 2h14a2 2 0 002-2v-4a2 2 0 00-2-2m-2-4h.01M17 16h.01"/>
            </svg>
            ServerSupervisor
          </h1>
        </div>

        <nav class="flex-1 px-4 space-y-1">
          <router-link to="/" class="nav-link" active-class="nav-link-active">
            <svg class="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M4 6a2 2 0 012-2h2a2 2 0 012 2v2a2 2 0 01-2 2H6a2 2 0 01-2-2V6zM14 6a2 2 0 012-2h2a2 2 0 012 2v2a2 2 0 01-2 2h-2a2 2 0 01-2-2V6zM4 16a2 2 0 012-2h2a2 2 0 012 2v2a2 2 0 01-2 2H6a2 2 0 01-2-2v-2zM14 16a2 2 0 012-2h2a2 2 0 012 2v2a2 2 0 01-2 2h-2a2 2 0 01-2-2v-2z"/>
            </svg>
            Dashboard
          </router-link>

          <router-link to="/docker" class="nav-link" active-class="nav-link-active">
            <svg class="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M20 7l-8-4-8 4m16 0l-8 4m8-4v10l-8 4m0-10L4 7m8 4v10M4 7v10l8 4"/>
            </svg>
            Docker
          </router-link>

          <router-link to="/apt" class="nav-link" active-class="nav-link-active">
            <svg class="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M4 4v5h.582m15.356 2A8.001 8.001 0 004.582 9m0 0H9m11 11v-5h-.581m0 0a8.003 8.003 0 01-15.357-2m15.357 2H15"/>
            </svg>
            APT Updates
          </router-link>

          <router-link to="/repos" class="nav-link" active-class="nav-link-active">
            <svg class="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M7 7h.01M7 3h5c.512 0 1.024.195 1.414.586l7 7a2 2 0 010 2.828l-7 7a2 2 0 01-2.828 0l-7-7A1.994 1.994 0 013 12V7a4 4 0 014-4z"/>
            </svg>
            Versions & Repos
          </router-link>
        </nav>

        <div class="p-4 border-t border-dark-700">
          <div class="flex items-center justify-between">
            <span class="text-sm text-gray-400">{{ auth.username }}</span>
            <button @click="handleLogout" class="text-gray-400 hover:text-red-400 transition-colors">
              <svg class="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M17 16l4-4m0 0l-4-4m4 4H7m6 4v1a3 3 0 01-3 3H6a3 3 0 01-3-3V7a3 3 0 013-3h4a3 3 0 013 3v1"/>
              </svg>
            </button>
          </div>
        </div>
      </aside>

      <!-- Main content -->
      <main class="flex-1 overflow-y-auto bg-dark-950 p-8">
        <router-view />
      </main>
    </div>

    <!-- Login page (no sidebar) -->
    <router-view v-else />
  </div>
</template>

<script setup>
import { useAuthStore } from './stores/auth'
import { useRouter } from 'vue-router'

const auth = useAuthStore()
const router = useRouter()

function handleLogout() {
  auth.logout()
  router.push('/login')
}
</script>

<style>
.nav-link {
  @apply flex items-center gap-3 px-4 py-2.5 rounded-lg text-gray-400 hover:text-gray-100 hover:bg-dark-800 transition-colors duration-200 text-sm font-medium;
}
.nav-link-active {
  @apply bg-primary-600/20 text-primary-400 hover:text-primary-300;
}
</style>
