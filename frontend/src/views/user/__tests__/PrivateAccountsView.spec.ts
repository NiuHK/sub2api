import { describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import PrivateAccountsView from '../PrivateAccountsView.vue'
import UserOpenAIAccountModal from '@/components/account/UserOpenAIAccountModal.vue'

const { list, get, remove } = vi.hoisted(() => ({ list: vi.fn(), get: vi.fn(), remove: vi.fn() }))
vi.mock('@/api/privateAccounts', () => ({
  privateAccountsAPI: { list, get, remove, create: vi.fn(), createPAT: vi.fn(), authURL: vi.fn(), createOAuth: vi.fn(), update: vi.fn() }
}))
vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return { ...actual, useI18n: () => ({ t: (key: string) => key }) }
})

const stub = (tag: string, slots: string[]) => ({
  template: `<${tag} data-test="${tag}">${slots.map(name => `<slot${name ? ` name="${name}"` : ''} />`).join('')}</${tag}>`
})

describe('PrivateAccountsView layout', () => {
  it('renders in the same AppLayout/TablePageLayout as admin accounts and opens the scoped modal', async () => {
    list.mockResolvedValue([])
    const wrapper = mount(PrivateAccountsView, { global: { stubs: {
      AppLayout: stub('div', ['']),
      TablePageLayout: stub('section', ['filters', 'table', 'pagination']),
      DataTable: true, Pagination: true, SearchInput: true, Select: true,
      AccountTableActions: { template: '<button data-test="create" @click="$emit(\'create\')">create</button>' },
      UserOpenAIAccountModal: true, ConfirmDialog: true,
      AccountStatusIndicator: true, AccountCapacityCell: true, AccountGroupsCell: true,
      PlatformTypeBadge: true, Icon: true
    } } })
    await flushPromises()
    expect(wrapper.find('[data-test="div"]').exists()).toBe(true)
    expect(wrapper.find('[data-test="section"]').exists()).toBe(true)
    expect(list).toHaveBeenCalled()
    await wrapper.find('[data-test="create"]').trigger('click')
    expect(wrapper.findAllComponents(UserOpenAIAccountModal)[0].props('show')).toBe(true)
  })
})
