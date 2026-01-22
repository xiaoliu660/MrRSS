<script setup lang="ts">
import { computed } from 'vue';
import type { Component } from 'vue';
import { PhInfo, PhWarning, PhWarningCircle } from '@phosphor-icons/vue';

interface Props {
  type?: 'info' | 'warning' | 'danger';
  icon?: Component;
  content: string;
}

const props = withDefaults(defineProps<Props>(), {
  type: 'info',
  icon: undefined,
});

const defaultIcon = computed(() => {
  switch (props.type) {
    case 'warning':
      return PhWarning;
    case 'danger':
      return PhWarningCircle;
    default:
      return PhInfo;
  }
});

const displayIcon = computed(() => props.icon || defaultIcon.value);

const boxClass = computed(() => {
  switch (props.type) {
    case 'warning':
      return 'info-box-warning';
    case 'danger':
      return 'info-box-danger';
    default:
      return 'info-box-info';
  }
});
</script>

<template>
  <div class="info-box" :class="boxClass">
    <component :is="displayIcon" :size="16" class="shrink-0 sm:w-5 sm:h-5" />
    <span class="text-xs sm:text-sm">{{ content }}</span>
  </div>
</template>

<style scoped>
.info-box {
  @apply flex items-center gap-2 sm:gap-3 py-2 sm:py-2.5 px-2.5 sm:px-3 rounded-lg;
}

.info-box-info {
  background-color: rgba(59, 130, 246, 0.05);
  border: 1px solid rgba(59, 130, 246, 0.3);
}

.info-box-info :deep(*) {
  @apply text-blue-500;
}

.info-box-warning {
  background-color: rgba(234, 179, 8, 0.05);
  border: 1px solid rgba(234, 179, 8, 0.3);
}

.info-box-warning :deep(*) {
  @apply text-yellow-500;
}

.info-box-danger {
  background-color: rgba(239, 68, 68, 0.05);
  border: 1px solid rgba(239, 68, 68, 0.3);
}

.info-box-danger :deep(*) {
  @apply text-red-500;
}
</style>
