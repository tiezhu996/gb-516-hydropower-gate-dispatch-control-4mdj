
import { computed, ref } from 'vue'; export function usePagination(total: () => number, initialPageSize = 20) { const page = ref(1); const pageSize = ref(initialPageSize); const pages = computed(() => Math.max(1, Math.ceil(total() / pageSize.value))); return { page, pageSize, pages }; }
