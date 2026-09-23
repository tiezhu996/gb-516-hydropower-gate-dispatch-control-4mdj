
import { request } from './client';
import type { DomainRecord } from '../types/domain';

export async function listGateUnit(page = 1, pageSize = 20, search = '') {
  return request<DomainRecord[]>(`/gates?page=${page}&pageSize=${pageSize}&search=${encodeURIComponent(search)}`);
}
export async function createGateUnit(input: Partial<DomainRecord>) {
  return request<DomainRecord>('/gates', { method: 'POST', body: JSON.stringify(input) });
}
export async function transitionGateUnit(id: number, status: string, expectedVersion: number, reason: string) {
  return request<DomainRecord>(`/gates/${id}/transition`, {
    method: 'POST', body: JSON.stringify({ status, expectedVersion, reason }),
  });
}
