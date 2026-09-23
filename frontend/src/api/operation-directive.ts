
import { request } from './client';
import type { DomainRecord } from '../types/domain';

export async function listOperationDirective(page = 1, pageSize = 20, search = '') {
  return request<DomainRecord[]>(`/directives?page=${page}&pageSize=${pageSize}&search=${encodeURIComponent(search)}`);
}
export async function createOperationDirective(input: Partial<DomainRecord>) {
  return request<DomainRecord>('/directives', { method: 'POST', body: JSON.stringify(input) });
}
export async function transitionOperationDirective(id: number, status: string, expectedVersion: number, reason: string) {
  return request<DomainRecord>(`/directives/${id}/transition`, {
    method: 'POST', body: JSON.stringify({ status, expectedVersion, reason }),
  });
}
