
<script setup lang="ts">import EntityPage from '../components/EntityPage.vue'; import { ENTITY_CONFIGS } from '../types/status'; import { useExecutionConfirmationStore } from '../stores/execution-confirmation'; const store = useExecutionConfirmationStore();</script>
<template><EntityPage :config="ENTITY_CONFIGS[3]" :store="store"/></template>
