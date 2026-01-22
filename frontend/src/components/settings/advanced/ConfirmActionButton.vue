<script setup lang="ts">
import { ref } from 'vue';
import type { Component } from 'vue';
import ButtonControl from '../base/SettingControl/ButtonControl.vue';

interface Props {
  label: string;
  confirmTitle: string;
  confirmMessage: string;
  isDanger?: boolean;
  loading?: boolean;
  icon?: Component;
}

const props = defineProps<Props>();

const emit = defineEmits<{
  confirm: [];
}>();

const isLoading = ref(false);

async function handleClick() {
  const confirmed = await window.showConfirm({
    title: props.confirmTitle,
    message: props.confirmMessage,
    isDanger: props.isDanger ?? false,
  });

  if (!confirmed) return;

  isLoading.value = true;
  try {
    await emit('confirm');
  } finally {
    isLoading.value = false;
  }
}
</script>

<template>
  <ButtonControl
    :icon="props.icon"
    :label="props.label"
    :type="props.isDanger ? 'danger' : 'secondary'"
    :loading="props.loading || isLoading"
    @click="handleClick"
  />
</template>
