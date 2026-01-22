<script setup lang="ts">
import { ref } from 'vue';

interface Template {
  name: string;
  [key: string]: any;
}

interface Props {
  templates: Array<Template>;
  label?: string;
}

defineProps<Props>();

const emit = defineEmits<{
  select: [template: Template];
}>();

const isOpen = ref(false);

function selectTemplate(template: Template) {
  emit('select', template);
  isOpen.value = false;
}

function toggleDropdown() {
  isOpen.value = !isOpen.value;
}

function closeDropdown() {
  isOpen.value = false;
}
</script>

<template>
  <div class="template-selector">
    <button type="button" class="template-selector-button" @click="toggleDropdown">
      {{ label || 'Select Template' }}
    </button>

    <Teleport to="body">
      <div v-if="isOpen" class="template-dropdown-overlay" @click="closeDropdown">
        <div class="template-dropdown" @click.stop>
          <button
            v-for="template in templates"
            :key="template.name"
            type="button"
            class="template-dropdown-item"
            @click="selectTemplate(template)"
          >
            {{ template.name }}
          </button>
        </div>
      </div>
    </Teleport>
  </div>
</template>

<style scoped>
.template-selector {
  position: relative;
  display: inline-block;
}

.template-selector-button {
  @apply bg-bg-tertiary border border-border text-text-primary px-3 sm:px-4 py-1.5 sm:py-2 rounded-md cursor-pointer flex items-center gap-1.5 sm:gap-2 font-medium hover:bg-bg-secondary transition-colors text-xs sm:text-sm;
}

.template-dropdown-overlay {
  position: fixed;
  inset: 0;
  z-index: 9999;
}

.template-dropdown {
  position: absolute;
  top: 100%;
  right: 0;
  margin-top: 0.25rem;
  z-index: 50;
  @apply bg-bg-secondary border border-border rounded-lg shadow-lg overflow-hidden min-w-[150px];
}

.template-dropdown-item {
  @apply w-full px-4 py-2 text-left hover:bg-bg-tertiary text-sm transition-colors;
}

.template-dropdown-item:first-child {
  @apply rounded-t-lg;
}

.template-dropdown-item:last-child {
  @apply rounded-b-lg;
}
</style>
