<template>
  <div class="space-y-5 p-4 md:p-6">
    <div class="flex flex-wrap items-center justify-between gap-3">
      <div>
        <h1 class="text-xl font-semibold text-gray-900 dark:text-white">{{ t('nav.accounts') }} · OpenAI</h1>
        <p class="mt-1 text-sm text-gray-500">账号只会绑定到你的 Private 分组，不能选择或修改分组。</p>
      </div>
      <button class="btn btn-primary" @click="openCreate">{{ t('admin.accounts.createAccount') }}</button>
    </div>
    <p v-if="error" role="alert" class="rounded-lg bg-red-50 p-3 text-sm text-red-700 dark:bg-red-900/20 dark:text-red-300">{{ error }}</p>
    <div class="overflow-x-auto rounded-xl border border-gray-200 bg-white dark:border-dark-700 dark:bg-dark-800">
      <table class="w-full min-w-[600px] text-left text-sm">
        <thead class="border-b border-gray-200 text-gray-500 dark:border-dark-700"><tr>
          <th class="p-4">{{ t('common.name') }}</th><th class="p-4">{{ t('admin.accounts.columns.type') }}</th>
          <th class="p-4">{{ t('admin.accounts.columns.status') }}</th><th class="p-4">{{ t('common.actions') }}</th>
        </tr></thead>
        <tbody>
          <tr v-for="account in accounts" :key="account.id" class="border-b border-gray-100 dark:border-dark-700">
            <td class="p-4 font-medium">{{ account.name }}</td><td class="p-4">{{ account.type }}</td>
            <td class="p-4"><AccountStatusIndicator :account="account" /></td>
            <td class="space-x-3 p-4">
              <button class="text-primary-600" @click="openEdit(account)">{{ t('common.edit') }}</button>
              <button class="text-red-600" @click="remove(account)">{{ t('common.delete') }}</button>
            </td>
          </tr>
          <tr v-if="!loading && !accounts.length"><td colspan="4" class="p-8 text-center text-gray-500">暂无账号</td></tr>
          <tr v-if="loading"><td colspan="4" class="p-8 text-center text-gray-500">{{ t('common.loading') }}</td></tr>
        </tbody>
      </table>
    </div>
    <div v-if="showForm" class="fixed inset-0 z-50 flex items-center justify-center bg-black/50 p-4" @click.self="showForm = false">
      <form class="w-full max-w-lg space-y-4 rounded-xl bg-white p-6 shadow-xl dark:bg-dark-800" @submit.prevent="save">
        <h2 class="text-lg font-semibold">{{ editing ? t('common.edit') : t('admin.accounts.createAccount') }}</h2>
        <label class="block text-sm">{{ t('common.name') }}<input v-model="name" required maxlength="100" class="input mt-1 w-full" /></label>
        <template v-if="!editing">
          <label class="block text-sm">接入方式
            <select v-model="method" class="input mt-1 w-full">
              <option value="apikey">OpenAI API Key</option><option value="oauth">OpenAI OAuth</option><option value="pat">Codex PAT</option>
            </select>
          </label>
          <p class="text-sm text-gray-500">目标分组：当前用户专属 Private 分组（自动绑定）</p>
        </template>
        <template v-if="method === 'apikey'">
          <label class="block text-sm">Base URL<input v-model="baseURL" class="input mt-1 w-full" placeholder="https://api.openai.com" /></label>
          <label class="block text-sm">API Key{{ editing ? '（留空则保留）' : '' }}<input v-model="secret" type="password" :required="!editing" autocomplete="off" class="input mt-1 w-full" /></label>
        </template>
        <label v-if="!editing && method === 'pat'" class="block text-sm">Codex PAT<input v-model="secret" type="password" required autocomplete="off" class="input mt-1 w-full" /></label>
        <template v-if="!editing && method === 'oauth'">
          <button type="button" class="btn btn-secondary" @click="startOAuth">生成 OpenAI 授权链接</button>
          <p v-if="oauthURL" class="break-all text-sm"><a :href="oauthURL" target="_blank" rel="noopener noreferrer" class="text-primary-600 underline">打开授权页面</a><br />{{ oauthURL }}</p>
          <label class="block text-sm">授权后的回调 URL（或 code）<input v-model="callback" class="input mt-1 w-full" required /></label>
        </template>
        <label class="block text-sm">并发数<input v-model.number="concurrency" type="number" min="0" max="100" class="input mt-1 w-full" :disabled="method === 'pat' && !editing" /></label>
        <div class="flex justify-end gap-2"><button type="button" class="btn btn-secondary" @click="showForm = false">{{ t('common.cancel') }}</button><button type="submit" class="btn btn-primary" :disabled="saving">{{ t('common.save') }}</button></div>
      </form>
    </div>
  </div>
</template>

<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import type { Account } from '@/types'
import AccountStatusIndicator from '@/components/account/AccountStatusIndicator.vue'
import { privateAccountsAPI } from '@/api/privateAccounts'
const { t } = useI18n()
const accounts = ref<Account[]>([])
const loading = ref(false)
const saving = ref(false)
const error = ref('')
const showForm = ref(false)
const editing = ref<Account | null>(null)
const name = ref('')
const method = ref<'apikey' | 'oauth' | 'pat'>('apikey')
const secret = ref('')
const baseURL = ref('https://api.openai.com')
const concurrency = ref(3)
const oauthURL = ref('')
const oauthSessionID = ref('')
const callback = ref('')
function message(e: unknown) { return e instanceof Error ? e.message : String(e) }
async function reload() {
  loading.value = true
  try { accounts.value = await privateAccountsAPI.list(); error.value = '' }
  catch (e) { error.value = message(e) }
  finally { loading.value = false }
}
function openCreate() {
  editing.value = null; method.value = 'apikey'; name.value = ''; secret.value = ''
  baseURL.value = 'https://api.openai.com'; concurrency.value = 3
  oauthURL.value = ''; oauthSessionID.value = ''; callback.value = ''; showForm.value = true
}
function openEdit(a: Account) {
  editing.value = a; method.value = a.type === 'apikey' ? 'apikey' : 'oauth'
  name.value = a.name; secret.value = ''; baseURL.value = String(a.credentials?.base_url || '')
  concurrency.value = a.concurrency; showForm.value = true
}
async function startOAuth() {
  try { const result = await privateAccountsAPI.authURL(); oauthURL.value = result.auth_url; oauthSessionID.value = result.session_id; error.value = '' }
  catch (e) { error.value = message(e) }
}
async function save() {
  saving.value = true; error.value = ''
  try {
    if (editing.value) {
      const credentials: Record<string, unknown> = {}
      if (method.value === 'apikey') {
        if (baseURL.value.trim()) credentials.base_url = baseURL.value.trim()
        if (secret.value.trim()) credentials.api_key = secret.value.trim()
      }
      await privateAccountsAPI.update(editing.value.id, { name: name.value.trim(), concurrency: concurrency.value, ...(Object.keys(credentials).length ? { credentials } : {}) })
    } else if (method.value === 'pat') {
      await privateAccountsAPI.createPAT(name.value.trim(), secret.value.trim())
    } else if (method.value === 'oauth') {
      if (!oauthSessionID.value) throw new Error('请先生成授权链接')
      let code = callback.value.trim(); let state = ''
      if (code.includes('?')) {
        const url = new URL(code)
        code = url.searchParams.get('code') || ''
        state = url.searchParams.get('state') || ''
      } else if (code.includes('#')) {
        const parts = code.split('#'); code = parts[0]; state = parts[1]
      }
      if (!state) state = new URL(oauthURL.value).searchParams.get('state') || ''
      if (!code || !state) throw new Error('回调中缺少 code 或 state')
      await privateAccountsAPI.createOAuth({ name: name.value.trim(), session_id: oauthSessionID.value, code, state })
    } else {
      await privateAccountsAPI.create({ name: name.value.trim(), type: 'apikey', concurrency: concurrency.value, credentials: { base_url: baseURL.value.trim(), api_key: secret.value.trim() } })
    }
    secret.value = ''; callback.value = ''; showForm.value = false; await reload()
  } catch (e) { error.value = message(e) }
  finally { saving.value = false }
}
async function remove(a: Account) {
  if (!confirm(`确定删除账号「${a.name}」吗？`)) return
  try { await privateAccountsAPI.remove(a.id); await reload() }
  catch (e) { error.value = message(e) }
}
onMounted(reload)
</script>
