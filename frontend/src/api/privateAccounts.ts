import { apiClient } from './client'
import type { Account } from '@/types'

const base = '/private-accounts'
export const privateAccountsAPI = {
  list: async (): Promise<Account[]> => (await apiClient.get<Account[]>(base)).data,
  get: async (id: number): Promise<Account> => (await apiClient.get<Account>(`${base}/${id}`)).data,
  create: async (input: { name: string; type: 'apikey' | 'oauth'; credentials: Record<string, unknown>; concurrency: number }): Promise<Account> =>
    (await apiClient.post<Account>(base, input)).data,
  createPAT: async (name: string, access_token: string): Promise<Account> =>
    (await apiClient.post<Account>(`${base}/codex-pat`, { name, access_token })).data,
  authURL: async (): Promise<{ auth_url: string; session_id: string }> =>
    (await apiClient.post<{ auth_url: string; session_id: string }>(`${base}/oauth/auth-url`, {})).data,
  createOAuth: async (input: { name: string; session_id: string; code: string; state: string }): Promise<Account> =>
    (await apiClient.post<Account>(`${base}/oauth/create`, input)).data,
  update: async (id: number, input: { name: string; credentials?: Record<string, unknown>; concurrency?: number }): Promise<Account> =>
    (await apiClient.put<Account>(`${base}/${id}`, input)).data,
  remove: async (id: number): Promise<void> => { await apiClient.delete(`${base}/${id}`) }
}
