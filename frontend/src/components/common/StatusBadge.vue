
<script setup lang="ts">import { computed } from 'vue'; import { statusLabel, statusTone } from '../../utils/format';
const props = defineProps<{ status: string }>(); const tone = computed(() => statusTone(props.status));</script>
<template><span :class="`status status--${tone}`">{{ statusLabel(status) }}</span></template>
