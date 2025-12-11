/**
 * Composable for validating settings fields
 */
import { computed, type Ref } from 'vue';
import type { SettingsData } from '@/types/settings';

export function useSettingsValidation(settings: Ref<SettingsData>) {
  /**
   * Check if refresh settings are valid
   */
  const isRefreshValid = computed(() => {
    if (settings.value.refresh_mode === 'fixed') {
      return settings.value.update_interval > 0;
    }
    return true;
  });

  /**
   * Check if translation settings are valid
   */
  const isTranslationValid = computed(() => {
    if (!settings.value.translation_enabled) {
      return true; // Not enabled, so no validation needed
    }

    if (settings.value.translation_provider === 'deepl') {
      return !!settings.value.deepl_api_key?.trim();
    } else if (settings.value.translation_provider === 'baidu') {
      return !!(settings.value.baidu_app_id?.trim() && settings.value.baidu_secret_key?.trim());
    } else if (settings.value.translation_provider === 'ai') {
      return !!settings.value.ai_api_key?.trim();
    }

    return true; // Google Translate doesn't need API key
  });

  /**
   * Check if summary settings are valid
   */
  const isSummaryValid = computed(() => {
    if (!settings.value.summary_enabled) {
      return true; // Not enabled, so no validation needed
    }

    if (settings.value.summary_provider === 'ai') {
      return !!settings.value.summary_ai_api_key?.trim();
    }
    return true; // Local summary doesn't need API key
  });

  /**
   * Check if proxy settings are valid
   */
  const isProxyValid = computed(() => {
    if (!settings.value.proxy_enabled) {
      return true; // Not enabled, so no validation needed
    }

    return !!(settings.value.proxy_host?.trim() && settings.value.proxy_port?.trim());
  });

  /**
   * Check if all settings are valid
   */
  const isValid = computed(() => {
    return (
      isRefreshValid.value && isTranslationValid.value && isSummaryValid.value && isProxyValid.value
    );
  });

  return {
    isRefreshValid,
    isTranslationValid,
    isSummaryValid,
    isProxyValid,
    isValid,
  };
}
