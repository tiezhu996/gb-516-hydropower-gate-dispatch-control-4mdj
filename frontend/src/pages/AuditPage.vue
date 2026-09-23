<script setup lang="ts">
import { onMounted, ref } from 'vue';
import { Refresh, Search } from '@element-plus/icons-vue';
import { listAudits, loadAuditSummary } from '../api/audit';
import type { AuditLog, AuditSummary } from '../types/domain';
import { formatDate } from '../utils/format';
import { usePolling } from '../hooks/usePolling';
import MetricCard from '../components/common/MetricCard.vue';

const logs = ref<AuditLog[]>([]);
const summary = ref<AuditSummary | null>(null);
const search = ref('');
const loading = ref(false);
const error = ref('');

async function load(): Promise<void> {
  loading.value = true;
  error.value = '';
  try {
    const [auditResult, summaryResult] = await Promise.all([listAudits(1, 50, search.value), loadAuditSummary(24)]);
    logs.value = auditResult.data;
    summary.value = summaryResult.data;
  } catch (reason) {
    error.value = reason instanceof Error ? reason.message : String(reason);
  } finally {
    loading.value = false;
  }
}

onMounted(() => void load());
usePolling(load, 30_000);
</script>

<template>
  <main class="workspace">
    <header class="page-header"><div><p class="eyebrow">治理与追踪</p><h1>操作审计</h1><p>按请求 ID 追踪操作者、实体和不可覆盖的状态迁移证据。</p></div></header>
    <section class="metrics" aria-label="审计统计">
      <MetricCard label="24 小时事件" :value="summary?.total || 0" detail="创建、更新与状态迁移" />
      <MetricCard label="状态迁移" :value="summary?.transitions || 0" detail="均与业务状态同事务" />
      <MetricCard label="活跃操作者" :value="summary?.uniqueActors || 0" detail="去重账号数" />
    </section>
    <section class="toolbar">
      <el-input v-model="search" :prefix-icon="Search" placeholder="搜索操作者、实体或动作" clearable @keyup.enter="load" />
      <el-button type="primary" :icon="Search" @click="load">查询</el-button>
      <el-button :icon="Refresh" @click="search = ''; load()">重置</el-button>
    </section>
    <el-alert v-if="error" :title="error" type="error" show-icon />
    <section v-loading="loading" class="audit-list" aria-live="polite">
      <article v-for="log in logs" :key="log.id">
        <time>{{ formatDate(log.createdAt) }}</time>
        <strong>{{ log.actor }} · {{ log.action }}</strong>
        <span>{{ log.entityType }} #{{ log.entityId }}</span>
        <code>{{ log.beforeState || '-' }} → {{ log.afterState || '-' }}</code>
        <small :title="log.requestId">{{ log.requestId }}</small>
      </article>
      <div v-if="!loading && !logs.length" class="empty">当前筛选范围内没有审计记录</div>
    </section>
  </main>
</template>
