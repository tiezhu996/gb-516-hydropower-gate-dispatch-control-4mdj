
<script setup lang="ts">
withDefaults(defineProps<{ modelValue: boolean; title: string; confirmLabel?: string; confirmDisabled?: boolean; loading?: boolean }>(), {
	confirmLabel: '确认', confirmDisabled: false, loading: false,
});
const emit = defineEmits<{ 'update:modelValue': [value: boolean]; confirm: [] }>();
</script>
<template>
  <el-dialog :model-value="modelValue" :title="title" width="min(620px, calc(100vw - 28px))" @close="emit('update:modelValue', false)">
    <slot />
    <template #footer><el-button @click="emit('update:modelValue', false)">取消</el-button><el-button type="primary" :disabled="confirmDisabled" :loading="loading" @click="emit('confirm')">{{ confirmLabel }}</el-button></template>
  </el-dialog>
</template>
