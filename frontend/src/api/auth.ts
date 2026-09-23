
import { request } from './client';
import type { UserSession } from '../types/domain';
export async function login(username: string, password: string): Promise<UserSession> {
	const session = (await request<Omit<UserSession, 'expiresAt'>>('/auth/login', { method: 'POST', body: JSON.stringify({ username, password }) })).data;
	return { ...session, expiresAt: Date.now() + session.expiresIn * 1000 };
}
