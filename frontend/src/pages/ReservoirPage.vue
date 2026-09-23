
<script setup lang="ts">import EntityPage from '../components/EntityPage.vue'; import { ENTITY_CONFIGS } from '../types/status'; import { useReservoirStore } from '../stores/reservoir'; const store = useReservoirStore();</script>
<template><EntityPage :config="ENTITY_CONFIGS[0]" :store="store"/></template>
