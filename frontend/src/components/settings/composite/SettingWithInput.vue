<script setup lang="ts">
import type { Component } from 'vue';
import SettingItem from '../base/SettingItem.vue';
import InputControl from '../base/SettingControl/InputControl.vue';

interface Props {
  icon?: Component;
  title: string;
  description?: string;
  modelValue: string;
  type?: 'text' | 'password' | 'email' | 'url';
  placeholder?: string;
  required?: boolean;
  disabled?: boolean;
  error?: boolean | string;
  width?: string;
}

defineProps<Props>();

const emit = defineEmits<{
  'update:modelValue': [value: string];
}>();
</script>

<template>
  <SettingItem :icon="icon" :title="title" :description="description" :required="required">
    <InputControl
      :model-value="modelValue"
      :type="type"
      :placeholder="placeholder"
      :disabled="disabled"
      :error="error"
      :width="width"
      @update:model-value="emit('update:modelValue', $event)"
    />
  </SettingItem>
</template>
