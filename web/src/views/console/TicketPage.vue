<script setup lang="ts">
import TicketDetailView from '../../components/TicketDetailView.vue'
</script>

<template>
  <TicketDetailView :id="Number($route.params.id)" />
</template>
