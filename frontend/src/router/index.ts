import { createRouter, createWebHistory } from 'vue-router';
import { bootstrapAuth, currentSession } from '../hooks/useAuth';
export const router = createRouter({ history: createWebHistory(), routes: [
	{ path: '/', redirect: '/reservoirs' },
	{ path: '/login', component: () => import('../pages/LoginPage.vue'), meta: { public: true } },
	{ path: '/reservoirs', component: () => import('../pages/ReservoirPage.vue') },
	{ path: '/gates', component: () => import('../pages/GateUnitPage.vue') },
	{ path: '/directives', component: () => import('../pages/OperationDirectivePage.vue') },
	{ path: '/confirmations', component: () => import('../pages/ExecutionConfirmationPage.vue') },
	{ path: '/audit', component: () => import('../pages/AuditPage.vue'), meta: { roles: ['reviewer', 'admin'] } },
] });

router.beforeEach(async (to) => {
  await bootstrapAuth();
	const active = currentSession();
	if (!active && !to.meta.public) return '/login';
	if (active && to.path === '/login') return '/reservoirs';
  const roles = to.meta.roles as string[] | undefined;
	if (roles && !roles.includes(active?.role || '')) return '/reservoirs';
  return true;
});
