
import { request } from './client';
import type { DomainRecord } from '../types/domain';

export async function listExecutionConfirmation(page = 1, pageSize = 20, search = '') {
  return request<DomainRecord[]>(`/confirmations?page=${page}&pageSize=${pageSize}&search=${encodeURIComponent(search)}`);
}
export async function createExecutionConfirmation(input: Partial<DomainRecord>) {
  return request<DomainRecord>('/confirmations', { method: 'POST', body: JSON.stringify(input) });
}
export async function transitionExecutionConfirmation(id: number, status: string, expectedVersion: number, reason: string) {
  return request<DomainRecord>(`/confirmations/${id}/transition`, {
    method: 'POST', body: JSON.stringify({ status, expectedVersion, reason }),
  });
}
