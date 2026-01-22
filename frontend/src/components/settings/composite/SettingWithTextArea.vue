<script setup lang="ts">
import type { Component } from 'vue';
import TextAreaControl from '../base/SettingControl/TextAreaControl.vue';

interface Props {
  icon?: Component;
  title: string;
  description?: string;
  modelValue: string;
  placeholder?: string;
  required?: boolean;
  disabled?: boolean;
  error?: boolean;
  rows?: number;
  resize?: boolean;
  fontMono?: boolean;
}

defineProps<Props>();

const emit = defineEmits<{
  'update:modelValue': [value: string];
}>();
</script>

<template>
  <div class="setting-item-textarea">
    <div class="flex items-center sm:items-start gap-2 sm:gap-3 min-w-0 mb-2">
      <component
        :is="icon"
        v-if="icon"
        :size="20"
        class="text-text-secondary mt-0.5 shrink-0 sm:w-6 sm:h-6"
      />
      <div class="flex-1 min-w-0">
        <div class="font-medium mb-0 sm:mb-1 text-sm sm:text-base">
          {{ title }} <span v-if="required" class="text-red-500">*</span>
        </div>
        <div v-if="description" class="text-xs text-text-secondary hidden sm:block">
          {{ description }}
        </div>
      </div>
    </div>
    <TextAreaControl
      :model-value="modelValue"
      :placeholder="placeholder"
      :disabled="disabled"
      :error="error"
      :rows="rows"
      :resize="resize"
      :font-mono="fontMono"
      @update:model-value="emit('update:modelValue', $event)"
    />
  </div>
</template>

<style scoped>
.setting-item-textarea {
  @apply flex flex-col gap-2 p-2 sm:p-3 rounded-lg bg-bg-secondary border border-border;
}
</style>
