<script setup lang="ts">
import type { Component } from 'vue';
import SettingItem from '../base/SettingItem.vue';
import ButtonControl from '../base/SettingControl/ButtonControl.vue';

interface Props {
  icon?: Component;
  title: string;
  description?: string;
  buttonIcon?: Component;
  buttonLabel?: string;
  buttonType?: 'primary' | 'secondary' | 'danger';
  required?: boolean;
  disabled?: boolean;
  loading?: boolean;
}

defineProps<Props>();

const emit = defineEmits<{
  click: [];
}>();
</script>

<template>
  <SettingItem :icon="icon" :title="title" :description="description" :required="required">
    <ButtonControl
      :icon="buttonIcon"
      :label="buttonLabel"
      :type="buttonType"
      :disabled="disabled"
      :loading="loading"
      @click="emit('click')"
    />
  </SettingItem>
</template>
