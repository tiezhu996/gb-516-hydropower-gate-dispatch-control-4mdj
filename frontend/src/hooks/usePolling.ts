import { onBeforeUnmount, onMounted, ref } from 'vue';

export function usePolling(callback: () => void | Promise<void>, intervalMs = 30_000) {
  const lastUpdatedAt = ref<Date | null>(null);
  let timer: number | undefined;

  async function run(): Promise<void> {
    if (document.visibilityState !== 'visible') return;
    await callback();
    lastUpdatedAt.value = new Date();
  }

  onMounted(() => {
    timer = window.setInterval(() => void run(), intervalMs);
  });
  onBeforeUnmount(() => {
    if (timer !== undefined) window.clearInterval(timer);
  });

  return { lastUpdatedAt, refresh: run };
}
