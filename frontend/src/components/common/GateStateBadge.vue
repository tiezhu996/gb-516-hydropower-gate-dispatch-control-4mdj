<script setup lang="ts">
import { computed } from 'vue';
import { Lock, Select, Switch, Unlock } from '@element-plus/icons-vue';
import type { GateState } from '../../types/status';

const props = defineProps<{ state: GateState | string }>();
const labels: Record<string, string> = { open: '开启', closed: '关闭', moving: '动作中', locked: '闭锁' };
const icon = computed(() => ({ open: Unlock, closed: Select, moving: Switch, locked: Lock }[props.state] || Lock));
</script>

<template>
  <span :class="['gate-state', `gate-state--${state}`]">
    <el-icon><component :is="icon" /></el-icon>{{ labels[state] || state }}
  </span>
</template>
