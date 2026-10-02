<template>
  <Teleport to="body">
    <div v-if="show" class="fixed inset-0 z-[10000] flex items-center justify-center bg-black/40 p-4" @click.self="$emit('close')">
      <div class="flex max-h-[calc(100vh-2rem)] w-full max-w-4xl flex-col rounded-xl bg-white p-5 shadow-xl dark:bg-dark-800">
        <div class="mb-4 flex items-center justify-between">
          <h2 class="text-lg font-semibold text-gray-900 dark:text-white">{{ t('admin.accounts.quotaShares.title') }}</h2>
          <button class="text-gray-500" @click="$emit('close')">&#x2715;</button>
        </div>

        <div class="mb-4 space-y-3">
          <p class="text-sm text-gray-600 dark:text-gray-300">{{ t('admin.accounts.quotaShares.description') }}</p>
          <div class="rounded-md border border-blue-100 bg-blue-50 px-3 py-2 text-xs leading-5 text-blue-800 dark:border-blue-900/60 dark:bg-blue-950/40 dark:text-blue-200">
            {{ t('admin.accounts.quotaShares.inputHint') }}
          </div>
          <div class="grid gap-2 rounded-md bg-gray-50 px-3 py-2 text-xs text-gray-600 dark:bg-dark-700 dark:text-gray-300 sm:grid-cols-2">
            <span>{{ t('admin.accounts.quotaShares.accountCurrentUsage') }}:</span>
            <span class="sm:text-right">{{ currentAccountUsage }}</span>
          </div>
        </div>

        <div class="mb-3 flex gap-2">
          <input
            v-model="search"
            class="min-w-0 flex-1 rounded-md border px-3 py-2 text-sm dark:border-dark-600 dark:bg-dark-700"
            :placeholder="t('admin.accounts.quotaShares.searchUsers')"
            @keyup.enter.prevent="loadUsers"
            @keydown.esc="showUserResults = false"
          />
          <button class="rounded-md bg-gray-100 px-3 py-2 text-sm disabled:cursor-not-allowed disabled:opacity-60 dark:bg-dark-700" :disabled="searchLoading" @click="loadUsers">
            {{ searchLoading ? t('common.loading') : t('common.search') }}
          </button>
        </div>

        <div v-if="showUserResults" class="mb-4 max-h-48 overflow-y-auto rounded-md border border-gray-200 bg-white dark:border-dark-600 dark:bg-dark-700">
          <div v-if="searchLoading" class="px-3 py-3 text-sm text-gray-500">{{ t('common.loading') }}</div>
          <template v-if="!searchLoading">
            <button
              v-for="user in users"
              :key="user.id"
              type="button"
              class="flex w-full items-center justify-between gap-3 border-b border-gray-100 px-3 py-2 text-left text-sm last:border-b-0 hover:bg-gray-100 disabled:cursor-default disabled:opacity-50 dark:border-dark-600 dark:hover:bg-dark-600"
              :disabled="isUserSelected(user.id)"
              @click="selectUser(user)"
            >
              <span class="min-w-0 truncate text-gray-800 dark:text-gray-100">{{ user.username || user.email }}</span>
              <span class="shrink-0 text-xs text-gray-500">{{ user.email }} (#{{ user.id }})</span>
            </button>
          </template>
          <div v-if="!searchLoading && !users.length" class="px-3 py-3 text-sm text-gray-500">{{ t('admin.accounts.quotaShares.noUsersFound') }}</div>
        </div>

        <div v-if="loading" class="py-8 text-center text-sm text-gray-500">{{ t('common.loading') }}</div>
        <div v-else class="min-h-0 flex-1 overflow-auto rounded-md border border-gray-200 dark:border-dark-600">
          <div class="min-w-[760px]">
            <div class="grid grid-cols-[minmax(260px,1fr)_180px_180px_36px] gap-3 border-b border-gray-200 bg-gray-50 px-3 py-2 text-xs font-semibold text-gray-600 dark:border-dark-600 dark:bg-dark-700 dark:text-gray-300">
              <span>{{ t('admin.accounts.quotaShares.userColumn') }}</span>
              <span>{{ t('admin.accounts.quotaShares.fiveHourLimit') }}</span>
              <span>{{ t('admin.accounts.quotaShares.sevenDayLimit') }}</span>
              <span aria-hidden="true"></span>
            </div>
            <div v-for="(row, index) in rows" :key="row.user_id" class="grid grid-cols-[minmax(260px,1fr)_180px_180px_36px] items-center gap-3 border-b border-gray-100 px-3 py-3 last:border-b-0 dark:border-dark-700">
              <div class="min-w-0">
                <div class="truncate text-sm font-medium text-gray-800 dark:text-gray-100">{{ userLabel(row) }}</div>
                <div class="truncate text-xs text-gray-500">{{ userEmail(row) }} · #{{ row.user_id }}</div>
              </div>
              <label class="space-y-1">
                <span class="block text-[11px] text-gray-500">{{ usageLabel(row.five_hour_used_percent) }}</span>
                <input v-model.number="row.five_hour_percent" type="number" min="-1" max="100" step="0.1" class="w-full rounded-md border px-2 py-1.5 text-sm dark:border-dark-600 dark:bg-dark-700" :aria-label="t('admin.accounts.quotaShares.fiveHourLimit')" />
              </label>
              <label class="space-y-1">
                <span class="block text-[11px] text-gray-500">{{ usageLabel(row.seven_day_used_percent) }}</span>
                <input v-model.number="row.seven_day_percent" type="number" min="-1" max="100" step="0.1" class="w-full rounded-md border px-2 py-1.5 text-sm dark:border-dark-600 dark:bg-dark-700" :aria-label="t('admin.accounts.quotaShares.sevenDayLimit')" />
              </label>
              <button class="text-red-500" :aria-label="t('common.delete')" @click="rows.splice(index, 1)">&#x2715;</button>
            </div>
            <div v-if="!rows.length" class="px-3 py-8 text-center text-sm text-gray-500">{{ t('admin.accounts.quotaShares.empty') }}</div>
          </div>
        </div>

        <div class="mt-5 flex justify-end gap-2">
          <button class="rounded-md px-4 py-2 text-sm" @click="$emit('close')">{{ t('common.cancel') }}</button>
          <button class="rounded-md bg-primary-600 px-4 py-2 text-sm text-white disabled:cursor-not-allowed disabled:opacity-60" :disabled="saving" @click="save">{{ saving ? t('common.saving') : t('common.save') }}</button>
        </div>
      </div>
    </div>
  </Teleport>
</template>

<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { adminAPI, type AccountQuotaShare } from '@/api/admin'
import type { Account, AdminUser } from '@/types'

const props = defineProps<{ show: boolean; account: Account | null }>()
const emit = defineEmits<{ close: []; saved: [] }>()
const { t } = useI18n()
const rows = ref<AccountQuotaShare[]>([])
const users = ref<AdminUser[]>([])
const search = ref('')
const loading = ref(false)
const searchLoading = ref(false)
const showUserResults = ref(false)
const saving = ref(false)

const formatPercent = (value: unknown) => {
  const number = typeof value === 'number' ? value : Number(value)
  return Number.isFinite(number) ? `${number.toFixed(1)}%` : t('admin.accounts.quotaShares.unavailable')
}

const currentAccountUsage = computed(() => {
  const extra = props.account?.extra
  if (!extra) return t('admin.accounts.quotaShares.unavailable')
  return `${t('admin.accounts.quotaShares.fiveHourShort')} ${formatPercent(extra.codex_5h_used_percent)} · ${t('admin.accounts.quotaShares.sevenDayShort')} ${formatPercent(extra.codex_7d_used_percent)}`
})

const userLabel = (row: AccountQuotaShare) => row.username || row.email || users.value.find(user => user.id === row.user_id)?.username || users.value.find(user => user.id === row.user_id)?.email || t('admin.accounts.quotaShares.unknownUser')
const userEmail = (row: AccountQuotaShare) => row.email || users.value.find(user => user.id === row.user_id)?.email || t('admin.accounts.quotaShares.emailUnavailable')
const isUserSelected = (id: number) => rows.value.some(row => row.user_id === id)
const usageLabel = (value: number | null | undefined) => value == null ? t('admin.accounts.quotaShares.currentUsageUnavailable') : `${t('admin.accounts.quotaShares.currentUsage')} ${formatPercent(value)}`

const loadUsers = async () => {
  searchLoading.value = true
  showUserResults.value = true
  try {
    const result = await adminAPI.users.list(1, 50, { search: search.value.trim() })
    users.value = result.items || []
  } catch (error) {
    users.value = []
    console.error('Failed to search users:', error)
  } finally {
    searchLoading.value = false
  }
}

const selectUser = (user: AdminUser) => {
  if (!isUserSelected(user.id)) {
    rows.value.push({ user_id: user.id, username: user.username, email: user.email, five_hour_percent: -1, seven_day_percent: -1 })
  }
  showUserResults.value = false
}

const load = async () => {
  if (!props.account) return
  loading.value = true
  showUserResults.value = false
  try {
    const result = await adminAPI.accounts.getQuotaShares(props.account.id)
    rows.value = result.shares || []
  } finally {
    loading.value = false
  }
}

const save = async () => {
  if (!props.account) return
  saving.value = true
  try {
    await adminAPI.accounts.updateQuotaShares(props.account.id, rows.value)
    emit('saved')
    emit('close')
  } finally {
    saving.value = false
  }
}

watch(() => props.show, visible => {
  if (visible) {
    search.value = ''
    users.value = []
    load()
  } else {
    showUserResults.value = false
  }
})
</script>
