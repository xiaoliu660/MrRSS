<script setup lang="ts">
interface Props {
  indent?: boolean;
}

withDefaults(defineProps<Props>(), {
  indent: true,
});
</script>

<template>
  <div
    class="nested-settings"
    :class="{
      'ml-2 sm:ml-4 pl-3 sm:pl-4 border-l-2 border-border': indent,
      'mt-2 sm:mt-3': true,
    }"
  >
    <div class="space-y-2 sm:space-y-3">
      <slot />
    </div>
  </div>
</template>

<style scoped>
.nested-settings {
  @apply w-full;
}
</style>
