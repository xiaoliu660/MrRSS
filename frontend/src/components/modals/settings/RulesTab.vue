<script setup>
import { store } from '../../../store.js';
import { ref, onMounted, computed, watch, onUnmounted } from 'vue';
import { 
    PhLightning, PhPlus, PhTrash, PhPencil, PhToggleLeft, PhToggleRight,
    PhPlay, PhFunnel, PhListChecks, PhCaretDown, PhCaretUp, PhInfo
} from "@phosphor-icons/vue";
import RuleEditorModal from '../RuleEditorModal.vue';

const props = defineProps({
    settings: { type: Object, required: true }
});

// Rules list
const rules = ref([]);

// Modal states
const showRuleEditor = ref(false);
const editingRule = ref(null);
const applyingRuleId = ref(null);

// Load rules from settings
onMounted(() => {
    loadRules();
});

function loadRules() {
    if (props.settings.rules) {
        try {
            const parsed = typeof props.settings.rules === 'string' 
                ? JSON.parse(props.settings.rules) 
                : props.settings.rules;
            rules.value = Array.isArray(parsed) ? parsed : [];
        } catch (e) {
            console.error('Error parsing rules:', e);
            rules.value = [];
        }
    }
}

// Watch for settings changes
watch(() => props.settings.rules, () => {
    loadRules();
}, { immediate: true });

// Save rules to settings
async function saveRules() {
    try {
        props.settings.rules = JSON.stringify(rules.value);
        await fetch('/api/settings', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ rules: props.settings.rules })
        });
    } catch (e) {
        console.error('Error saving rules:', e);
    }
}

// Add new rule
function addRule() {
    editingRule.value = null;
    showRuleEditor.value = true;
}

// Edit existing rule
function editRule(rule) {
    editingRule.value = { ...rule };
    showRuleEditor.value = true;
}

// Delete rule
async function deleteRule(ruleId) {
    const confirmed = await window.showConfirm({
        title: store.i18n.t('ruleDeleteConfirmTitle'),
        message: store.i18n.t('ruleDeleteConfirmMessage'),
        confirmText: store.i18n.t('delete'),
        cancelText: store.i18n.t('cancel'),
        isDanger: true
    });
    
    if (!confirmed) return;
    
    rules.value = rules.value.filter(r => r.id !== ruleId);
    await saveRules();
    window.showToast(store.i18n.t('ruleDeletedSuccess'), 'success');
}

// Toggle rule enabled state
async function toggleRuleEnabled(rule) {
    rule.enabled = !rule.enabled;
    await saveRules();
}

// Save rule from editor
async function handleSaveRule(rule) {
    if (editingRule.value) {
        // Update existing rule
        const index = rules.value.findIndex(r => r.id === rule.id);
        if (index !== -1) {
            rules.value[index] = rule;
        }
    } else {
        // Add new rule
        rule.id = Date.now();
        rule.enabled = true;
        rules.value.push(rule);
    }
    
    await saveRules();
    showRuleEditor.value = false;
    window.showToast(store.i18n.t('ruleSavedSuccess'), 'success');
}

// Apply rule now
async function applyRule(rule) {
    applyingRuleId.value = rule.id;
    
    try {
        const res = await fetch('/api/rules/apply', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify(rule)
        });
        
        if (res.ok) {
            const data = await res.json();
            window.showToast(store.i18n.t('ruleAppliedSuccess', { count: data.affected }), 'success');
            store.fetchArticles();
            store.fetchUnreadCounts();
        } else {
            window.showToast(store.i18n.t('errorSavingSettings'), 'error');
        }
    } catch (e) {
        console.error('Error applying rule:', e);
        window.showToast(store.i18n.t('errorSavingSettings'), 'error');
    } finally {
        applyingRuleId.value = null;
    }
}

// Format condition for display
function formatCondition(rule) {
    if (!rule.conditions || rule.conditions.length === 0) {
        return store.i18n.t('conditionAlways');
    }
    
    // Simplified display - show first condition
    const first = rule.conditions[0];
    let text = formatSingleCondition(first);
    
    if (rule.conditions.length > 1) {
        text += ` ${store.i18n.t('andNMore', { count: rule.conditions.length - 1 })}`;
    }
    
    return text;
}

function formatSingleCondition(condition) {
    const fieldLabels = {
        'feed_name': store.i18n.t('feedName'),
        'feed_category': store.i18n.t('feedCategory'),
        'article_title': store.i18n.t('articleTitle'),
        'published_after': store.i18n.t('publishedAfter'),
        'published_before': store.i18n.t('publishedBefore'),
        'is_read': store.i18n.t('readStatus'),
        'is_favorite': store.i18n.t('favoriteStatus')
    };
    
    const field = fieldLabels[condition.field] || condition.field;
    let value = condition.value || (condition.values && condition.values.length > 0 ? condition.values[0] : '');
    
    if (condition.negate) {
        return `${store.i18n.t('not')} ${field}: ${value}`;
    }
    
    return `${field}: ${value}`;
}

// Format actions for display
function formatActions(rule) {
    if (!rule.actions || rule.actions.length === 0) {
        return '-';
    }
    
    const actionLabels = {
        'favorite': store.i18n.t('actionFavorite'),
        'unfavorite': store.i18n.t('actionUnfavorite'),
        'hide': store.i18n.t('actionHide'),
        'unhide': store.i18n.t('actionUnhide'),
        'mark_read': store.i18n.t('actionMarkRead'),
        'mark_unread': store.i18n.t('actionMarkUnread')
    };
    
    return rule.actions.map(a => actionLabels[a] || a).join(', ');
}
</script>

<template>
    <div class="space-y-3">
        <div class="flex items-center justify-between mb-2">
            <div class="flex items-center gap-2">
                <PhLightning :size="18" class="text-text-secondary" />
                <div>
                    <h3 class="font-semibold text-sm">{{ store.i18n.t('rules') }}</h3>
                    <p class="text-xs text-text-secondary">{{ store.i18n.t('rulesDesc') }}</p>
                </div>
            </div>
            <button @click="addRule" class="btn-primary text-xs py-1 px-2">
                <PhPlus :size="12" />
                {{ store.i18n.t('addRule') }}
            </button>
        </div>
        
        <!-- Rules List -->
        <div v-if="rules.length === 0" class="empty-state">
            <PhLightning :size="48" class="mx-auto mb-3 opacity-50" />
            <p class="text-text-secondary">{{ store.i18n.t('noRules') }}</p>
            <p class="text-text-secondary text-xs mt-1">{{ store.i18n.t('noRulesHint') }}</p>
        </div>
        
        <div v-else class="space-y-2">
            <div v-for="rule in rules" :key="rule.id" class="rule-card">
                <div class="flex items-start gap-3">
                    <!-- Enable toggle -->
                    <button @click="toggleRuleEnabled(rule)" class="toggle-btn" :title="rule.enabled ? store.i18n.t('ruleEnabled') : store.i18n.t('ruleDisabled')">
                        <PhToggleRight v-if="rule.enabled" :size="24" class="text-accent" />
                        <PhToggleLeft v-else :size="24" class="text-text-secondary" />
                    </button>
                    
                    <!-- Rule info -->
                    <div class="flex-1 min-w-0">
                        <div class="font-medium text-sm truncate" :class="{ 'text-text-secondary': !rule.enabled }">
                            {{ rule.name || store.i18n.t('rules') + ' #' + rule.id }}
                        </div>
                        <div class="text-xs text-text-secondary mt-1 flex flex-wrap items-center gap-1">
                            <span class="condition-badge">
                                <PhFunnel :size="10" />
                                {{ formatCondition(rule) }}
                            </span>
                            <span class="text-text-tertiary">→</span>
                            <span class="action-badge">
                                <PhListChecks :size="10" />
                                {{ formatActions(rule) }}
                            </span>
                        </div>
                    </div>
                    
                    <!-- Actions -->
                    <div class="flex items-center gap-1 shrink-0">
                        <button 
                            @click="applyRule(rule)" 
                            class="action-btn" 
                            :disabled="applyingRuleId === rule.id"
                            :title="store.i18n.t('applyRuleNow')"
                        >
                            <PhPlay v-if="applyingRuleId !== rule.id" :size="14" />
                            <span v-else class="animate-spin">⟳</span>
                        </button>
                        <button @click="editRule(rule)" class="action-btn" :title="store.i18n.t('editRule')">
                            <PhPencil :size="14" />
                        </button>
                        <button @click="deleteRule(rule.id)" class="action-btn danger" :title="store.i18n.t('deleteRule')">
                            <PhTrash :size="14" />
                        </button>
                    </div>
                </div>
            </div>
        </div>
        
        <!-- Tip -->
        <div class="tip-box">
            <PhInfo :size="14" class="text-accent shrink-0" />
            <span>{{ store.i18n.t('conditionIf') }} + {{ store.i18n.t('ruleCondition') }} → {{ store.i18n.t('ruleActions') }}</span>
        </div>
        
        <!-- Rule Editor Modal -->
        <RuleEditorModal 
            v-if="showRuleEditor"
            :show="showRuleEditor"
            :rule="editingRule"
            @close="showRuleEditor = false"
            @save="handleSaveRule"
        />
    </div>
</template>

<style scoped>
.btn-primary {
    @apply bg-accent text-white border-none rounded cursor-pointer flex items-center gap-1 font-medium hover:bg-accent-hover transition-colors;
}

.empty-state {
    @apply text-center py-8;
}

.rule-card {
    @apply p-3 rounded-lg bg-bg-secondary border border-border;
}

.toggle-btn {
    @apply p-0 bg-transparent border-none cursor-pointer transition-colors mt-0.5;
}

.condition-badge, .action-badge {
    @apply inline-flex items-center gap-1 px-1.5 py-0.5 rounded text-[10px] bg-bg-tertiary;
}

.action-btn {
    @apply p-1.5 rounded-md bg-transparent border-none cursor-pointer text-text-secondary hover:bg-bg-tertiary hover:text-text-primary transition-colors;
}

.action-btn.danger:hover {
    @apply text-red-500 bg-red-500/10;
}

.action-btn:disabled {
    @apply opacity-50 cursor-not-allowed;
}

.tip-box {
    @apply flex items-center gap-2 text-xs text-text-secondary py-1.5 px-2.5 rounded-md;
    background-color: rgba(59, 130, 246, 0.05);
    border: 1px solid rgba(59, 130, 246, 0.3);
}

.animate-spin {
    animation: spin 1s linear infinite;
}

@keyframes spin {
    from { transform: rotate(0deg); }
    to { transform: rotate(360deg); }
}
</style>
