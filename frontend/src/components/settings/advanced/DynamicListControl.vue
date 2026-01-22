<script setup lang="ts">
import { ref, watch } from 'vue';
import { PhPlus, PhTrash } from '@phosphor-icons/vue';

interface KeyValuePair {
  id: string;
  key: string;
  value: string;
}

interface Props {
  modelValue: Array<{ key: string; value: string }>;
  keyPlaceholder?: string;
  valuePlaceholder?: string;
  keyLabel?: string;
  valueLabel?: string;
  addButtonText?: string;
  removeButtonAriaLabel?: string;
  maxItems?: number;
}

const props = withDefaults(defineProps<Props>(), {
  keyPlaceholder: 'Key',
  valuePlaceholder: 'Value',
  keyLabel: '',
  valueLabel: '',
  addButtonText: 'Add',
  removeButtonAriaLabel: 'Remove',
  maxItems: 20,
});

/* eslint-disable */
const emit = defineEmits<{
  (e: 'update:modelValue', value: Array<{ key: string; value: string }>): void;
}>();
/* eslint-enable */

// Convert modelValue to internal format with IDs
const items = ref<KeyValuePair[]>(
  props.modelValue.map((item, index) => ({
    id: `${Date.now()}-${index}`,
    key: item.key,
    value: item.value,
  }))
);

// Watch for external changes
watch(
  () => props.modelValue,
  (newValue) => {
    const hasNewEntries =
      newValue.length !== items.value.length ||
      newValue.some(
        (item, index) =>
          items.value[index]?.key !== item.key || items.value[index]?.value !== item.value
      );

    if (hasNewEntries) {
      items.value = newValue.map((item, index) => ({
        id: `${Date.now()}-${index}`,
        key: item.key,
        value: item.value,
      }));
    }
  },
  { deep: true }
);

function addItem() {
  if (items.value.length >= props.maxItems) return;
  items.value.push({
    id: `${Date.now()}`,
    key: '',
    value: '',
  });
  emitUpdate();
}

function removeItem(id: string) {
  items.value = items.value.filter((item) => item.id !== id);
  emitUpdate();
}

function updateItemKey(id: string, newKey: string) {
  const item = items.value.find((i) => i.id === id);
  if (item) {
    item.key = newKey;
    emitUpdate();
  }
}

function updateItemValue(id: string, newValue: string) {
  const item = items.value.find((i) => i.id === id);
  if (item) {
    item.value = newValue;
    emitUpdate();
  }
}

function emitUpdate() {
  const cleanItems = items.value
    .filter((item) => item.key.trim() || item.value.trim())
    .map((item) => ({
      key: item.key,
      value: item.value,
    }));
  emit('update:modelValue', cleanItems);
}
</script>

<template>
  <div class="dynamic-list-control">
    <div class="dynamic-list-items space-y-1.5 sm:space-y-2">
      <div
        v-for="item in items"
        :key="item.id"
        class="dynamic-list-item flex items-center gap-1.5 sm:gap-2"
      >
        <input
          :value="item.key"
          type="text"
          :placeholder="keyPlaceholder"
          class="input-field flex-1 text-xs sm:text-sm"
          @input="(e) => updateItemKey(item.id, (e.target as HTMLInputElement).value)"
        />
        <input
          :value="item.value"
          type="text"
          :placeholder="valuePlaceholder"
          class="input-field flex-1 text-xs sm:text-sm"
          @input="(e) => updateItemValue(item.id, (e.target as HTMLInputElement).value)"
        />
        <button
          type="button"
          class="p-1.5 sm:p-2 rounded hover:bg-red-50 dark:hover:bg-red-900/20 text-text-secondary hover:text-red-500 transition-all shrink-0"
          :title="removeButtonAriaLabel"
          @click="removeItem(item.id)"
        >
          <PhTrash :size="14" class="sm:w-4 sm:h-4" />
        </button>
      </div>
    </div>

    <button type="button" class="add-button" :disabled="items.length >= maxItems" @click="addItem">
      <PhPlus :size="14" class="sm:w-4 sm:h-4" />
      <span>{{ addButtonText }}</span>
    </button>
  </div>
</template>

<style scoped>
.input-field {
  @apply p-1.5 sm:p-2.5 border border-border rounded-md bg-bg-secondary text-text-primary focus:border-accent focus:outline-none transition-colors;
}

.input-field::placeholder {
  @apply text-text-secondary;
}

.add-button {
  @apply w-full p-1.5 sm:p-2 rounded border border-dashed border-border text-text-secondary hover:border-accent hover:text-accent transition-all text-xs font-medium flex items-center justify-center gap-1.5 sm:gap-2 disabled:opacity-50 disabled:cursor-not-allowed mt-2 sm:mt-3;
}

.add-button:hover {
  background-color: rgba(var(--accent-rgb, 59 130 246), 0.05);
}
</style>
