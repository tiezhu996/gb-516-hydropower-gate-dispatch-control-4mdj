
import { defineStore } from 'pinia';
import { ApiError, request } from '../api/client';
import type { DomainRecord, PageMeta } from '../types/domain';

function describeError(error: unknown): string {
	if (error instanceof ApiError) {
		if (error.code === 'gate_occupied' && error.occupiedByDirective) {
			return `闸门执行权已被指令 ${error.occupiedByDirective} 占用，当前指令保持原状态，待对方完成或中止后再开工`;
		}
		return error.message;
	}
	return error instanceof Error ? error.message : String(error);
}

export function createEntityStore(id: string) {
  return defineStore(id, {
    state: () => ({ items: [] as DomainRecord[], meta: { page: 1, pageSize: 20, total: 0 } as PageMeta, loading: false, error: '' }),
    actions: {
      async load(path: string, search = '') { this.loading = true; this.error = ''; try { const result = await request<DomainRecord[]>(`/${path}?page=1&pageSize=20&search=${encodeURIComponent(search)}`); this.items = result.data; this.meta = result.meta || { page: 1, pageSize: 20, total: result.data.length }; } catch (error) { this.error = describeError(error); } finally { this.loading = false; } },
		async createRecord(path: string, input: Partial<DomainRecord>) { this.loading = true; this.error = ''; try { await request<DomainRecord>(`/${path}`, { method: 'POST', body: JSON.stringify(input) }); await this.load(path); } catch (error) { this.error = describeError(error); } finally { this.loading = false; } },
		async transition(path: string, item: DomainRecord, status: string, reason: string) { this.loading = true; this.error = ''; try { await request<DomainRecord>(`/${path}/${item.id}/transition`, { method: 'POST', body: JSON.stringify({ status, expectedVersion: item.version, reason }) }); await this.load(path); } catch (error) { this.error = describeError(error); } finally { this.loading = false; } },
    },
  });
}
