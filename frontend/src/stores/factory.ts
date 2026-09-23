
import { defineStore } from 'pinia';
import { request } from '../api/client';
import type { DomainRecord, PageMeta } from '../types/domain';
export function createEntityStore(id: string) {
  return defineStore(id, {
    state: () => ({ items: [] as DomainRecord[], meta: { page: 1, pageSize: 20, total: 0 } as PageMeta, loading: false, error: '' }),
    actions: {
      async load(path: string, search = '') { this.loading = true; this.error = ''; try { const result = await request<DomainRecord[]>(`/${path}?page=1&pageSize=20&search=${encodeURIComponent(search)}`); this.items = result.data; this.meta = result.meta || { page: 1, pageSize: 20, total: result.data.length }; } catch (error) { this.error = error instanceof Error ? error.message : String(error); } finally { this.loading = false; } },
		async createRecord(path: string, input: Partial<DomainRecord>) { this.loading = true; this.error = ''; try { await request<DomainRecord>(`/${path}`, { method: 'POST', body: JSON.stringify(input) }); await this.load(path); } catch (error) { this.error = error instanceof Error ? error.message : String(error); } finally { this.loading = false; } },
		async transition(path: string, item: DomainRecord, status: string, reason: string) { this.loading = true; this.error = ''; try { await request<DomainRecord>(`/${path}/${item.id}/transition`, { method: 'POST', body: JSON.stringify({ status, expectedVersion: item.version, reason }) }); await this.load(path); } catch (error) { this.error = error instanceof Error ? error.message : String(error); } finally { this.loading = false; } },
    },
  });
}
