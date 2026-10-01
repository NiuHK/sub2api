import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { enableAutoUnmount, flushPromises, mount } from '@vue/test-utils'
import type { AdminGroup } from '@/types'
import GroupRateMultipliersModal from '../GroupRateMultipliersModal.vue'

const mocks = vi.hoisted(() => ({ list: vi.fn(), getGroupRateMultipliers: vi.fn(), batchSetGroupRateMultipliers: vi.fn() }))
vi.mock('@/api/admin', () => ({ adminAPI: { users: { list: mocks.list }, groups: mocks } }))
vi.mock('@/stores/app', () => ({ useAppStore: () => ({ showSuccess: vi.fn(), showError: vi.fn() }) }))
vi.mock('vue-i18n', () => ({ useI18n: () => ({ t: (key: string) => key }) }))
enableAutoUnmount(afterEach)
afterEach(() => vi.useRealTimers())
beforeEach(() => {
  vi.clearAllMocks()
  vi.useFakeTimers()
  mocks.getGroupRateMultipliers.mockResolvedValue([])
  mocks.list.mockResolvedValue({ items: [{ id: 7, email: 'user@example.com', status: 'active' }] })
  mocks.batchSetGroupRateMultipliers.mockResolvedValue(undefined)
})

async function selectUser() {
  const wrapper = mount(GroupRateMultipliersModal, {
    props: { show: false, group: { id: 1, name: 'Group', platform: 'openai' } as AdminGroup },
    global: { stubs: {
      BaseDialog: { props: ['show'], template: '<div v-if="show"><slot /></div>' },
      Icon: true, PlatformIcon: true, Pagination: true
    } }
  })
  await wrapper.setProps({ show: true })
  await flushPromises()
  await wrapper.get('input[type="text"]').setValue('user')
  await vi.advanceTimersByTimeAsync(300)
  await flushPromises()
  await wrapper.findAll('button').find(b => b.text().includes('user@example.com'))!.trigger('click')
  return wrapper
}

describe('GroupRateMultipliersModal new override validation', () => {
  it.each(['', '0', '-1'])('does not add invalid rate %j', async (value) => {
    const wrapper = await selectUser()
    const input = wrapper.get('input[placeholder="1.0"]')
    await input.setValue('100')
    await input.setValue(value)
    const add = wrapper.findAll('button').find(b => b.text() === 'common.add')!
    expect(add.attributes('disabled')).toBeDefined()
    await add.trigger('click')
    expect(wrapper.find('tbody tr').exists()).toBe(false)
    expect(mocks.batchSetGroupRateMultipliers).not.toHaveBeenCalled()
  })

  it.each([0.25, 1])('saves positive rate %s', async (value) => {
    const wrapper = await selectUser()
    await wrapper.get('input[placeholder="1.0"]').setValue(String(value))
    await wrapper.findAll('button').find(b => b.text() === 'common.add')!.trigger('click')
    await wrapper.findAll('button').find(b => b.text() === 'common.save')!.trigger('click')
    await flushPromises()
    expect(mocks.batchSetGroupRateMultipliers).toHaveBeenCalledWith(1, [{
      user_id: 7,
      rate_multiplier: value,
      quota_percentage_5h_enabled: false,
      quota_percentage_7d_enabled: false
    }])
  })

  it('saves quota percentage and both window switches', async () => {
    const wrapper = await selectUser()
    await wrapper.get('input[placeholder="1.0"]').setValue('1')
    await wrapper.get('input[placeholder="admin.groups.quotaPercentage"]').setValue('25')
    const switches = wrapper.findAll('[role="switch"]')
    expect(switches).toHaveLength(2)
    await switches[0].trigger('click')
    await switches[1].trigger('click')
    await wrapper.findAll('button').find(b => b.text() === 'common.add')!.trigger('click')
    await wrapper.findAll('button').find(b => b.text() === 'common.save')!.trigger('click')
    await flushPromises()

    expect(mocks.batchSetGroupRateMultipliers).toHaveBeenCalledWith(1, [{
      user_id: 7,
      rate_multiplier: 1,
      quota_percentage: 25,
      quota_percentage_5h_enabled: true,
      quota_percentage_7d_enabled: true
    }])
  })

  it('preserves a quota-only entry whose rate multiplier is null', async () => {
    mocks.getGroupRateMultipliers.mockResolvedValueOnce([{
      user_id: 7,
      user_name: '',
      user_email: 'user@example.com',
      user_notes: '',
      user_status: 'active',
      rate_multiplier: null,
      quota_percentage: 30,
      quota_percentage_5h_enabled: true,
      quota_percentage_7d_enabled: false
    }])

    const wrapper = mount(GroupRateMultipliersModal, {
      props: { show: false, group: { id: 1, name: 'Group', platform: 'openai' } as AdminGroup },
      global: { stubs: {
        BaseDialog: { props: ['show'], template: '<div v-if="show"><slot /></div>' },
        Icon: true, PlatformIcon: true, Pagination: true
      } }
    })
    await wrapper.setProps({ show: true })
    await flushPromises()
    const quotaInput = wrapper.get('input[placeholder="不限"]')
    await quotaInput.setValue('31')
    await quotaInput.trigger('change')
    await wrapper.findAll('button').find(b => b.text() === 'common.save')!.trigger('click')
    await flushPromises()

    expect(mocks.batchSetGroupRateMultipliers).toHaveBeenCalledWith(1, [{
      user_id: 7,
      rate_multiplier: null,
      quota_percentage: 31,
      quota_percentage_5h_enabled: true,
      quota_percentage_7d_enabled: false
    }])
  })
})
