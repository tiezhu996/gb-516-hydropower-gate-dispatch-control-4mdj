
import { request } from './client';
import type { AuditLog, AuditSummary } from '../types/domain';
export async function listAudits(page = 1, pageSize = 30, search = '') {
	return request<AuditLog[]>(`/audits?page=${page}&pageSize=${pageSize}&search=${encodeURIComponent(search)}`);
}
export async function loadOverview() { return request<Record<string, Record<string, number>>>('/overview'); }
export async function loadAuditSummary(windowHours = 24) { return request<AuditSummary>(`/audit-summary?windowHours=${windowHours}`); }
