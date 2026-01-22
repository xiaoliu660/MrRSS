<script setup lang="ts">
import type { Component } from 'vue';
import SettingItem from '../base/SettingItem.vue';
import NumberControl from '../base/SettingControl/NumberControl.vue';

interface Props {
  icon?: Component;
  title: string;
  description?: string;
  modelValue: number;
  min?: number;
  max?: number;
  step?: number;
  suffix?: string;
  placeholder?: string;
  required?: boolean;
  disabled?: boolean;
  error?: boolean;
  width?: string;
}

defineProps<Props>();

const emit = defineEmits<{
  'update:modelValue': [value: number];
}>();
</script>

<template>
  <SettingItem :icon="icon" :title="title" :description="description" :required="required">
    <NumberControl
      :model-value="modelValue"
      :min="min"
      :max="max"
      :step="step"
      :suffix="suffix"
      :placeholder="placeholder"
      :disabled="disabled"
      :error="error"
      :width="width"
      @update:model-value="emit('update:modelValue', $event)"
    />
  </SettingItem>
</template>
