
<script setup lang="ts">
import type { DomainRecord } from '../../types/domain';
import { formatDate } from '../../utils/format';
import StatusBadge from './StatusBadge.vue';

defineProps<{ records: DomainRecord[]; kind: string }>();
</script>

<template>
  <section class="timeline-panel" aria-label="指令与执行轨迹">
    <header><strong>{{ kind === 'operationDirective' ? '双人确认轨迹' : '关联指令执行回执' }}</strong><span>最近 {{ Math.min(records.length, 3) }} 条</span></header>
    <div v-if="records.length" class="directive-timeline">
      <article v-for="item in records.slice(0, 3)" :key="item.id">
        <div class="timeline-record"><strong>{{ item.code }}</strong><span>{{ item.relatedCode || '未关联指令' }}</span><StatusBadge :status="item.status" /></div>
        <ol v-if="kind === 'operationDirective'" class="approval-steps">
          <li v-for="approval in item.approvals || []" :key="approval.id">
            <span>{{ approval.stage === 'submitted' ? '提交' : '复核' }}</span>
            <strong>{{ approval.actor }}</strong>
            <small>{{ formatDate(approval.createdAt) }}</small>
          </li>
          <li v-if="!(item.approvals || []).length" class="pending-step">等待值班员提交</li>
        </ol>
        <p v-else>{{ item.confirmedBy ? `${item.confirmedBy} 于 ${formatDate(item.confirmedAt || '')}确认` : item.evidence }}</p>
      </article>
    </div>
    <div v-else class="empty">暂无可展示的调度证据</div>
  </section>
</template>
