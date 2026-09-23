
<script setup lang="ts">defineProps<{ label: string; value: string | number; detail?: string }>();</script>
<template><section class="metric"><span>{{ label }}</span><strong>{{ value }}</strong><small v-if="detail">{{ detail }}</small></section></template>
