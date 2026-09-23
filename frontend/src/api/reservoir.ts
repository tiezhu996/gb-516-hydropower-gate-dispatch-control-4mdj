
import { request } from './client';
import type { DomainRecord } from '../types/domain';

export async function listReservoir(page = 1, pageSize = 20, search = '') {
  return request<DomainRecord[]>(`/reservoirs?page=${page}&pageSize=${pageSize}&search=${encodeURIComponent(search)}`);
}
export async function createReservoir(input: Partial<DomainRecord>) {
  return request<DomainRecord>('/reservoirs', { method: 'POST', body: JSON.stringify(input) });
}
export async function transitionReservoir(id: number, status: string, expectedVersion: number, reason: string) {
  return request<DomainRecord>(`/reservoirs/${id}/transition`, {
    method: 'POST', body: JSON.stringify({ status, expectedVersion, reason }),
  });
}
