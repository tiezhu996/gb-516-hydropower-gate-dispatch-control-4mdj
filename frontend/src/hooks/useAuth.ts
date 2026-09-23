import { computed, readonly, ref } from 'vue';
import { clearSession, getToken, loadSession, request, saveSession } from '../api/client';
import { login } from '../api/auth';
import type { SessionResponse, UserSession } from '../types/domain';

const session = ref<UserSession | null>(loadSession());
const loading = ref(false);
let bootstrapPromise: Promise<UserSession | null> | null = null;
let bootstrapped = false;

export async function bootstrapAuth(): Promise<UserSession | null> {
	if (bootstrapped) return session.value;
	if (bootstrapPromise) return bootstrapPromise;
	const stored = loadSession();
	if (!stored || !getToken()) {
		session.value = null;
		bootstrapped = true;
		return null;
	}
	loading.value = true;
	bootstrapPromise = request<SessionResponse>('/session').then((result) => {
		const next = { ...stored, ...result.data };
		saveSession(next);
		session.value = next;
		return next;
	}).catch(() => {
		clearSession();
		session.value = null;
		return null;
	}).finally(() => {
		bootstrapped = true;
		loading.value = false;
		bootstrapPromise = null;
	});
	return bootstrapPromise;
}

export async function authenticate(username: string, password: string): Promise<UserSession> {
	loading.value = true;
	try {
		const next = await login(username.trim(), password);
		saveSession(next);
		session.value = next;
		bootstrapped = true;
		return next;
	} finally {
		loading.value = false;
	}
}

export function currentSession(): UserSession | null {
	return session.value;
}

export function roleIs(...roles: string[]): boolean {
	return Boolean(session.value && roles.includes(session.value.role));
}

export function useAuth() {
	return {
		session: readonly(session),
		loading: readonly(loading),
		authenticated: computed(() => Boolean(session.value && getToken())),
		can: (...roles: string[]) => roleIs(...roles),
		authenticate,
		logout: () => {
			clearSession();
			session.value = null;
			bootstrapped = true;
		},
	};
}
