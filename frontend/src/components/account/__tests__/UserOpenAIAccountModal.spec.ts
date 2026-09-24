import { beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import UserOpenAIAccountModal from '../UserOpenAIAccountModal.vue'

const { create, createPAT, createOAuth, authURL, update } = vi.hoisted(() => ({
  create: vi.fn(), createPAT: vi.fn(), createOAuth: vi.fn(), authURL: vi.fn(), update: vi.fn()
}))
vi.mock('@/api/privateAccounts', () => ({ privateAccountsAPI: { create, createPAT, createOAuth, authURL, update } }))
vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return { ...actual, useI18n: () => ({ t: (key: string) => key }) }
})
const baseDialogStub = {
  props: ['show', 'title'],
  template: '<div v-if="show" data-test="account-dialog"><h2>{{ title }}</h2><slot /><slot name="footer" /></div>'
}
function render() {
  return mount(UserOpenAIAccountModal, { props: { show: true }, global: { stubs: { BaseDialog: baseDialogStub, Icon: true } } })
}
const choose = async (wrapper: ReturnType<typeof render>, title: string) => {
  const button = wrapper.findAll('button').find(b => b.text().includes(title))
  expect(button).toBeDefined()
  await button!.trigger('click')
}

describe('UserOpenAIAccountModal', () => {
  beforeEach(() => { vi.clearAllMocks() })
  it('copies the admin dialog form without a group selector and creates API Key via user API', async () => {
    create.mockResolvedValue({ id: 1 })
    const wrapper = render()
    expect(wrapper.find('[data-test="account-dialog"]').exists()).toBe(true)
    expect(wrapper.find('[data-tour="account-form-groups"]').exists()).toBe(false)
    await choose(wrapper, 'API Key')
    await wrapper.find('#user-account-name').setValue('Personal upstream')
    await wrapper.find('#user-account-api-key').setValue('sk-test')
    await wrapper.find('form').trigger('submit')
    await flushPromises()
    expect(create).toHaveBeenCalledWith({ name: 'Personal upstream', type: 'apikey', concurrency: 3,
      credentials: { base_url: 'https://api.openai.com', api_key: 'sk-test' } })
    expect(wrapper.emitted('saved')).toHaveLength(1)
  })
  it('routes PAT through the validating user endpoint', async () => {
    createPAT.mockResolvedValue({ id: 2 })
    const wrapper = render()
    await choose(wrapper, 'Codex PAT')
    await wrapper.find('#user-account-name').setValue('PAT')
    await wrapper.find('#user-account-pat').setValue('at-example')
    await wrapper.find('form').trigger('submit')
    await flushPromises()
    expect(createPAT).toHaveBeenCalledWith('PAT', 'at-example')
  })
  it('keeps OAuth inside the same dialog and submits the callback state', async () => {
    authURL.mockResolvedValue({ auth_url: 'https://auth.openai.com/authorize?state=own-state', session_id: 'session' })
    createOAuth.mockResolvedValue({ id: 3 })
    const wrapper = render()
    await wrapper.find('#user-account-name').setValue('OAuth')
    await wrapper.find('form').trigger('submit')
    await flushPromises()
    expect(authURL).toHaveBeenCalledTimes(1)
    await wrapper.find('#user-account-callback').setValue('http://localhost:1455/auth/callback?code=own-code&state=own-state')
    await wrapper.find('form').trigger('submit')
    await flushPromises()
    expect(createOAuth).toHaveBeenCalledWith({ name: 'OAuth', session_id: 'session', code: 'own-code', state: 'own-state' })
  })
})
