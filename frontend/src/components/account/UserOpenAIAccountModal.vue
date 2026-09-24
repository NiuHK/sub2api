<template>
  <BaseDialog
    :show="show"
    :title="t(account ? 'admin.accounts.editAccount' : 'admin.accounts.createAccount')"
    width="wide"
    @close="emit('close')"
  >
    <div v-if="!account && method === 'oauth'" class="mb-6 flex items-center justify-center gap-4">
      <div class="flex items-center gap-2">
        <span :class="['flex h-8 w-8 items-center justify-center rounded-full text-sm font-semibold', 'bg-primary-500 text-white']">1</span>
        <span class="text-sm font-medium text-gray-700 dark:text-gray-300">{{ t('admin.accounts.oauth.authMethod') }}</span>
      </div>
      <div class="h-0.5 w-8 bg-gray-300 dark:bg-dark-600" />
      <div class="flex items-center gap-2">
        <span :class="['flex h-8 w-8 items-center justify-center rounded-full text-sm font-semibold', step === 2 ? 'bg-primary-500 text-white' : 'bg-gray-200 text-gray-500 dark:bg-dark-600']">2</span>
        <span class="text-sm font-medium text-gray-700 dark:text-gray-300">{{ t('admin.accounts.oauth.openai.title') }}</span>
      </div>
    </div>

    <form id="user-openai-account-form" class="space-y-5" @submit.prevent="submit">
      <template v-if="step === 1">
        <div>
          <label class="input-label" for="user-account-name">{{ t('admin.accounts.accountName') }}</label>
          <input id="user-account-name" v-model="name" type="text" maxlength="100" required class="input" :placeholder="t('admin.accounts.enterAccountName')" />
        </div>
        <div v-if="!account">
          <label class="input-label">{{ t('admin.accounts.platform') }}</label>
          <div class="mt-2 rounded-lg bg-gray-100 p-1 dark:bg-dark-700">
            <div class="flex items-center justify-center gap-2 rounded-md bg-white px-4 py-2.5 text-sm font-medium text-green-600 shadow-sm dark:bg-dark-600 dark:text-green-400">OpenAI</div>
          </div>
        </div>
        <div v-if="!account">
          <label class="input-label">{{ t('admin.accounts.accountType') }}</label>
          <div class="mt-2 grid grid-cols-1 gap-3 sm:grid-cols-3">
            <button
              v-for="option in methods" :key="option.id" type="button"
              :class="['flex items-center gap-3 rounded-lg border-2 p-3 text-left transition-all', method === option.id ? 'border-green-500 bg-green-50 dark:bg-green-900/20' : 'border-gray-200 hover:border-green-300 dark:border-dark-600 dark:hover:border-green-700']"
              @click="method = option.id"
            >
              <span :class="['flex h-8 w-8 shrink-0 items-center justify-center rounded-lg', method === option.id ? 'bg-green-500 text-white' : 'bg-gray-100 text-gray-500 dark:bg-dark-600 dark:text-gray-400']"><Icon name="key" size="sm" /></span>
              <span class="min-w-0"><span class="block text-sm font-medium text-gray-900 dark:text-white">{{ option.title }}</span><span class="block text-xs text-gray-500 dark:text-gray-400">{{ option.caption }}</span></span>
            </button>
          </div>
        </div>
        <div v-if="!account" class="rounded-lg border border-gray-200 bg-gray-50 p-3 text-sm text-gray-600 dark:border-dark-600 dark:bg-dark-700 dark:text-gray-300">
          {{ t('admin.accounts.columns.groups') }}：{{ t('privateAccounts.managedGroup') }}
        </div>
        <div v-if="method === 'apikey'" class="space-y-4">
          <div><label class="input-label" for="user-account-base-url">{{ t('admin.accounts.baseUrl') }}</label><input id="user-account-base-url" v-model="baseURL" class="input" placeholder="https://api.openai.com" /></div>
          <div><label class="input-label" for="user-account-api-key">API Key{{ account ? t('privateAccounts.keepApiKey') : '' }}</label><input id="user-account-api-key" v-model="secret" type="password" autocomplete="off" class="input" :required="!account" /></div>
        </div>
        <div v-if="method === 'pat' && !account"><label class="input-label" for="user-account-pat">Codex PAT</label><input id="user-account-pat" v-model="secret" type="password" autocomplete="off" required class="input" /></div>
        <div v-if="method !== 'pat' || account"><label class="input-label" for="user-account-concurrency">{{ t('admin.accounts.concurrency') }}</label><input id="user-account-concurrency" v-model.number="concurrency" type="number" min="0" max="100" class="input" /></div>
      </template>
      <template v-else>
        <div class="rounded-lg border border-gray-200 bg-gray-50 p-4 dark:border-dark-600 dark:bg-dark-700">
          <p class="mb-3 text-sm text-gray-600 dark:text-gray-300">{{ t('admin.accounts.oauth.authMethod') }} · OpenAI</p>
          <a v-if="oauthURL" :href="oauthURL" target="_blank" rel="noopener noreferrer" class="btn btn-secondary inline-flex">{{ t('privateAccounts.openAuth') }}</a>
          <p v-if="oauthURL" class="mt-3 break-all text-xs text-gray-500">{{ oauthURL }}</p>
        </div>
        <div><label class="input-label" for="user-account-callback">{{ t('privateAccounts.callback') }}</label><input id="user-account-callback" v-model="callback" required class="input" /></div>
      </template>
      <p v-if="error" role="alert" class="text-sm text-red-600 dark:text-red-400">{{ error }}</p>
    </form>
    <template #footer>
      <div class="flex w-full justify-end gap-3">
        <button v-if="step === 2" type="button" class="btn btn-secondary mr-auto" @click="step = 1">{{ t('common.back') }}</button>
        <button type="button" class="btn btn-secondary" @click="emit('close')">{{ t('common.cancel') }}</button>
        <button type="submit" form="user-openai-account-form" class="btn btn-primary" :disabled="saving">
          {{ saving ? t('admin.accounts.creating') : step === 1 && method === 'oauth' && !account ? t('common.next') : account ? t('common.save') : t('common.create') }}
        </button>
      </div>
    </template>
  </BaseDialog>
</template>

<script setup lang="ts">
import { ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import BaseDialog from '@/components/common/BaseDialog.vue'
import Icon from '@/components/icons/Icon.vue'
import { privateAccountsAPI } from '@/api/privateAccounts'
import type { Account } from '@/types'

type Method = 'apikey' | 'oauth' | 'pat'
const props = defineProps<{ show: boolean; account?: Account | null }>()
const emit = defineEmits<{ (e: 'close'): void; (e: 'saved'): void }>()
const { t } = useI18n()
const methods: { id: Method; title: string; caption: string }[] = [
  { id: 'oauth', title: 'OAuth', caption: 'ChatGPT / Codex' },
  { id: 'apikey', title: 'API Key', caption: 'OpenAI API' },
  { id: 'pat', title: 'Codex PAT', caption: 'Personal Access Token' }
]
const name = ref('')
const method = ref<Method>('oauth')
const secret = ref('')
const baseURL = ref('https://api.openai.com')
const concurrency = ref(3)
const oauthURL = ref('')
const oauthSessionID = ref('')
const callback = ref('')
const step = ref(1)
const saving = ref(false)
const error = ref('')
watch(() => [props.show, props.account] as const, () => {
  if (!props.show) return
  name.value = props.account?.name ?? ''
  method.value = props.account?.type === 'apikey' ? 'apikey' : 'oauth'
  secret.value = ''
  baseURL.value = String(props.account?.credentials?.base_url || 'https://api.openai.com')
  concurrency.value = props.account?.concurrency ?? 3
  oauthURL.value = ''; oauthSessionID.value = ''; callback.value = ''; step.value = 1; error.value = ''
}, { immediate: true })
function message(e: unknown): string { return e instanceof Error ? e.message : String(e) }
async function submit() {
  saving.value = true; error.value = ''
  try {
    if (props.account) {
      const credentials: Record<string, unknown> = {}
      if (method.value === 'apikey') {
        if (baseURL.value.trim()) credentials.base_url = baseURL.value.trim()
        if (secret.value.trim()) credentials.api_key = secret.value.trim()
      }
      await privateAccountsAPI.update(props.account.id, { name: name.value.trim(), concurrency: concurrency.value, ...(Object.keys(credentials).length ? { credentials } : {}) })
    } else if (method.value === 'apikey') {
      await privateAccountsAPI.create({ name: name.value.trim(), type: 'apikey', concurrency: concurrency.value, credentials: { base_url: baseURL.value.trim(), api_key: secret.value.trim() } })
    } else if (method.value === 'pat') {
      await privateAccountsAPI.createPAT(name.value.trim(), secret.value.trim())
    } else if (step.value === 1) {
      const result = await privateAccountsAPI.authURL()
      oauthURL.value = result.auth_url; oauthSessionID.value = result.session_id; step.value = 2
      return
    } else {
      let code = callback.value.trim(); let state = ''
      if (code.includes('?')) { const url = new URL(code); code = url.searchParams.get('code') || ''; state = url.searchParams.get('state') || '' }
      else if (code.includes('#')) { const parts = code.split('#'); code = parts[0]; state = parts[1] }
      if (!state) state = new URL(oauthURL.value).searchParams.get('state') || ''
      if (!code || !state) throw new Error(t('privateAccounts.missingCallback'))
      await privateAccountsAPI.createOAuth({ name: name.value.trim(), session_id: oauthSessionID.value, code, state })
    }
    secret.value = ''; callback.value = ''; emit('saved')
  } catch (e) { error.value = message(e) }
  finally { saving.value = false }
}
</script>
