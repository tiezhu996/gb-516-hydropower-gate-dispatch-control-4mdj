
<script setup lang="ts">import EntityPage from '../components/EntityPage.vue'; import { ENTITY_CONFIGS } from '../types/status'; import { useGateUnitStore } from '../stores/gate-unit'; const store = useGateUnitStore();</script>
<template><EntityPage :config="ENTITY_CONFIGS[1]" :store="store"/></template>
