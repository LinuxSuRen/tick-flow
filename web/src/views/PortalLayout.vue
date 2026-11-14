<script setup lang="ts">
import AppHeader from '../components/AppHeader.vue'
import { notificationsStore } from '../stores/notifications'
import { onMounted, onUnmounted } from 'vue'

onMounted(() => notificationsStore.startPolling())
onUnmounted(() => notificationsStore.stopPolling())
</script>

<template>
  <div class="portal">
    <AppHeader />
    <main class="page">
      <router-view />
    </main>
  </div>
</template>
