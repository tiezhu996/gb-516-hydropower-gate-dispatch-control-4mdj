<script setup lang="ts">
import { reactive, ref } from 'vue';
import { useRouter } from 'vue-router';
import { Lock, Right, User } from '@element-plus/icons-vue';
import { useAuth } from '../hooks/useAuth';

const router = useRouter();
const { authenticate, loading } = useAuth();
const form = reactive({ username: '', password: '' });
const error = ref('');

async function submit(): Promise<void> {
	if (!form.username.trim() || form.password.length < 6) {
		error.value = '请输入有效的账号和密码';
		return;
	}
	error.value = '';
	try {
		await authenticate(form.username, form.password);
		await router.replace('/reservoirs');
	} catch (reason) {
		error.value = reason instanceof Error ? reason.message : '登录失败';
	}
}
</script>

<template>
	<main class="login-page">
		<section class="login-panel" aria-labelledby="login-title">
			<header>
				<span>CONTROL DESK</span>
				<h1 id="login-title">水电站闸门调度许可</h1>
				<p>安全运行工作台</p>
			</header>
			<el-alert v-if="error" :title="error" type="error" show-icon :closable="false" />
			<el-form label-position="top" @submit.prevent="submit">
				<el-form-item label="账号">
					<el-input v-model="form.username" :prefix-icon="User" autocomplete="username" autofocus />
				</el-form-item>
				<el-form-item label="密码">
					<el-input v-model="form.password" :prefix-icon="Lock" type="password" autocomplete="current-password" show-password @keyup.enter="submit" />
				</el-form-item>
				<el-button type="primary" native-type="submit" :icon="Right" :loading="loading" :disabled="!form.username.trim() || form.password.length < 6">登录</el-button>
			</el-form>
		</section>
	</main>
</template>
