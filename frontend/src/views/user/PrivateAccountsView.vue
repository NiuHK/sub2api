<template>
  <AppLayout>
    <TablePageLayout>
      <template #filters>
        <div class="flex flex-wrap-reverse items-start justify-between gap-3">
          <div class="flex flex-wrap items-center gap-3">
            <SearchInput v-model="search" :placeholder="t('admin.accounts.searchAccounts')" class="w-full sm:w-64" />
            <Select v-model="typeFilter" class="w-40" :options="typeOptions" />
            <Select v-model="statusFilter" class="w-40" :options="statusOptions" />
          </div>
          <AccountTableActions :loading="loading" @refresh="reload" @create="showCreate = true" />
        </div>
        <p v-if="error" role="alert" class="mt-3 rounded-lg border border-red-200 bg-red-50 px-4 py-2 text-sm text-red-700 dark:border-red-900/50 dark:bg-red-900/20 dark:text-red-300">{{ error }}</p>
      </template>
      <template #table>
        <div class="flex min-h-0 flex-1 flex-col overflow-hidden">
          <DataTable
            :columns="columns"
            :data="pageAccounts"
            :loading="loading"
            row-key="id"
            default-sort-key="name"
            default-sort-order="asc"
            sort-storage-key="private-openai-account-sort"
          >
            <template #cell-name="{ row }"><span class="font-medium text-gray-900 dark:text-white">{{ row.name }}</span></template>
            <template #cell-platform_type="{ row }">
              <PlatformTypeBadge :platform="row.platform" :type="row.type" :auth-mode="String(row.credentials?.auth_mode || '')" />
            </template>
            <template #cell-capacity="{ row }"><AccountCapacityCell :account="row" /></template>
            <template #cell-status="{ row }"><AccountStatusIndicator :account="row" /></template>
            <template #cell-groups="{ row }"><AccountGroupsCell :groups="row.groups || []" :max-display="4" /></template>
            <template #cell-last_used_at="{ value }"><span class="text-sm text-gray-500 dark:text-dark-400">{{ formatRelativeTime(value) }}</span></template>
            <template #cell-created_at="{ value }"><span class="text-sm text-gray-500 dark:text-dark-400">{{ formatDateTime(value) }}</span></template>
            <template #cell-actions="{ row }">
              <div class="flex items-center gap-1">
                <button type="button" class="flex flex-col items-center gap-0.5 rounded-lg p-1.5 text-gray-500 transition-colors hover:bg-gray-100 hover:text-primary-600 dark:hover:bg-dark-700 dark:hover:text-primary-400" @click="editAccount(row)">
                  <Icon name="edit" size="sm" /><span class="text-xs">{{ t('common.edit') }}</span>
                </button>
                <button type="button" class="flex flex-col items-center gap-0.5 rounded-lg p-1.5 text-gray-500 transition-colors hover:bg-red-50 hover:text-red-600 dark:hover:bg-red-900/20 dark:hover:text-red-400" @click="deletingAccount = row">
                  <Icon name="trash" size="sm" /><span class="text-xs">{{ t('common.delete') }}</span>
                </button>
              </div>
            </template>
          </DataTable>
        </div>
      </template>
      <template #pagination>
        <Pagination v-if="filteredAccounts.length > 0" :page="page" :page-size="pageSize" :total="filteredAccounts.length" @update:page="page = $event" @update:pageSize="changePageSize" />
      </template>
    </TablePageLayout>
    <UserOpenAIAccountModal :show="showCreate" @close="showCreate = false" @saved="accountSaved" />
    <UserOpenAIAccountModal :show="!!editingAccount" :account="editingAccount" @close="editingAccount = null" @saved="accountSaved" />
    <ConfirmDialog
      :show="!!deletingAccount"
      :title="t('admin.accounts.deleteAccount')"
      :message="t('admin.accounts.deleteConfirm', { name: deletingAccount?.name })"
      :confirm-text="t('common.delete')"
      :cancel-text="t('common.cancel')"
      :danger="true"
      @confirm="confirmDelete"
      @cancel="deletingAccount = null"
    />
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import AppLayout from '@/components/layout/AppLayout.vue'
import TablePageLayout from '@/components/layout/TablePageLayout.vue'
import DataTable from '@/components/common/DataTable.vue'
import Pagination from '@/components/common/Pagination.vue'
import SearchInput from '@/components/common/SearchInput.vue'
import Select from '@/components/common/Select.vue'
import ConfirmDialog from '@/components/common/ConfirmDialog.vue'
import PlatformTypeBadge from '@/components/common/PlatformTypeBadge.vue'
import Icon from '@/components/icons/Icon.vue'
import AccountTableActions from '@/components/admin/account/AccountTableActions.vue'
import AccountStatusIndicator from '@/components/account/AccountStatusIndicator.vue'
import AccountCapacityCell from '@/components/account/AccountCapacityCell.vue'
import AccountGroupsCell from '@/components/account/AccountGroupsCell.vue'
import UserOpenAIAccountModal from '@/components/account/UserOpenAIAccountModal.vue'
import { privateAccountsAPI } from '@/api/privateAccounts'
import { formatDateTime, formatRelativeTime } from '@/utils/format'
import type { Account } from '@/types'
import type { Column } from '@/components/common/types'

const { t } = useI18n()
const accounts = ref<Account[]>([])
const loading = ref(false)
const error = ref('')
const search = ref('')
const typeFilter = ref('')
const statusFilter = ref('')
const page = ref(1)
const pageSize = ref(20)
const showCreate = ref(false)
const editingAccount = ref<Account | null>(null)
const deletingAccount = ref<Account | null>(null)
const typeOptions = computed(() => [
  { value: '', label: t('admin.accounts.allTypes') },
  { value: 'oauth', label: t('admin.accounts.oauthType') },
  { value: 'apikey', label: t('admin.accounts.apiKey') }
])
const statusOptions = computed(() => [
  { value: '', label: t('admin.accounts.allStatus') },
  { value: 'active', label: t('admin.accounts.status.active') },
  { value: 'inactive', label: t('admin.accounts.status.inactive') },
  { value: 'error', label: t('admin.accounts.status.error') }
])
const columns = computed<Column[]>(() => [
  { key: 'name', label: t('admin.accounts.columns.name'), sortable: true },
  { key: 'id', label: t('admin.accounts.columns.id'), sortable: true },
  { key: 'platform_type', label: t('admin.accounts.columns.platformType'), sortable: false },
  { key: 'capacity', label: t('admin.accounts.columns.capacity'), sortable: false },
  { key: 'status', label: t('admin.accounts.columns.status'), sortable: true },
  { key: 'groups', label: t('admin.accounts.columns.groups'), sortable: false },
  { key: 'last_used_at', label: t('admin.accounts.columns.lastUsed'), sortable: true },
  { key: 'created_at', label: t('admin.accounts.columns.createdAt'), sortable: true },
  { key: 'actions', label: t('admin.accounts.columns.actions'), sortable: false }
])
const filteredAccounts = computed(() => accounts.value.filter(a =>
  (!search.value || `${a.name} ${a.id}`.toLowerCase().includes(search.value.trim().toLowerCase())) &&
  (!typeFilter.value || a.type === typeFilter.value) &&
  (!statusFilter.value || a.status === statusFilter.value)
))
const pageAccounts = computed(() => filteredAccounts.value.slice((page.value - 1) * pageSize.value, page.value * pageSize.value))
watch([search, typeFilter, statusFilter], () => { page.value = 1 })
function changePageSize(size: number) { pageSize.value = size; page.value = 1 }
function message(e: unknown) { return e instanceof Error ? e.message : String(e) }
async function reload() {
  loading.value = true
  try {
    accounts.value = await privateAccountsAPI.list()
    if (page.value > Math.max(1, Math.ceil(filteredAccounts.value.length / pageSize.value))) page.value = 1
    error.value = ''
  } catch (e) { error.value = message(e) }
  finally { loading.value = false }
}
async function editAccount(account: Account) {
  try { editingAccount.value = await privateAccountsAPI.get(account.id) }
  catch (e) { error.value = message(e) }
}
function accountSaved() { showCreate.value = false; editingAccount.value = null; void reload() }
async function confirmDelete() {
  if (!deletingAccount.value) return
  try { await privateAccountsAPI.remove(deletingAccount.value.id); deletingAccount.value = null; await reload() }
  catch (e) { error.value = message(e) }
}
onMounted(reload)
</script>
