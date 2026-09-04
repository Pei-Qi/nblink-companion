<script setup lang="ts">
import type { Component } from 'vue'

defineProps<{
  icon: Component
  label: string
  disabled?: boolean
  active?: boolean
  danger?: boolean
}>()

defineEmits<{ click: [] }>()
</script>

<template>
  <button
    class="icon-button"
    :class="{ active, danger }"
    type="button"
    :disabled="disabled"
    :aria-label="label"
    :title="label"
    @click="$emit('click')"
  >
    <component :is="icon" :size="18" :stroke-width="1.8" aria-hidden="true" />
  </button>
</template>
