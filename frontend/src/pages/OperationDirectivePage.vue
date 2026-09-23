
<script setup lang="ts">import EntityPage from '../components/EntityPage.vue'; import { ENTITY_CONFIGS } from '../types/status'; import { useOperationDirectiveStore } from '../stores/operation-directive'; const store = useOperationDirectiveStore();</script>
<template><EntityPage :config="ENTITY_CONFIGS[2]" :store="store"/></template>
