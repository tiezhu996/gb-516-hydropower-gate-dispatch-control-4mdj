
<script setup lang="ts">
import { useRouter } from 'vue-router';
import { Connection, SwitchButton } from '@element-plus/icons-vue';
import { useAuth } from './hooks/useAuth';

const router = useRouter();
const { session, loading, can, logout } = useAuth();
const navigation = [
  { to: '/reservoirs', label: '库区' },
  { to: '/gates', label: '闸门' },
  { to: '/directives', label: '操作指令' },
  { to: '/confirmations', label: '执行确认' },
  { to: '/audit', label: '审计记录', roles: ['reviewer', 'admin'] },
];

async function signOut(): Promise<void> {
	logout();
	await router.replace('/login');
}
</script>

<template>
  <div v-if="loading" class="app-loading">正在建立安全会话...</div>
	<router-view v-else-if="!session" />
  <div v-else class="app-shell">
    <aside>
      <div class="brand"><span>CONTROL DESK</span><strong>水电站闸门调度许可</strong></div>
      <nav aria-label="主导航">
        <router-link
          v-for="item in navigation.filter((entry) => !entry.roles || can(...entry.roles))"
          :key="item.to"
          :to="item.to"
        >{{ item.label }}</router-link>
      </nav>
      <div class="user-panel">
        <span>{{ session?.displayName }}</span>
        <small>{{ session?.role }}</small>
		<el-button class="account-switch" text :icon="SwitchButton" @click="signOut">退出登录</el-button>
      </div>
    </aside>
    <section class="content">
      <header class="topbar">
        <span>闸门运行许可台</span>
        <span class="live-dot"><el-icon><Connection /></el-icon>服务已连接</span>
      </header>
      <router-view />
    </section>
  </div>
</template>
