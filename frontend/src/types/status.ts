import type { EntityConfig } from './domain';

export type GateState = 'open' | 'closed' | 'moving' | 'locked';
export const ALL_GATE_STATE: readonly GateState[] = ['open', 'closed', 'moving', 'locked'];
export type DirectiveState = 'draft' | 'pending' | 'approved' | 'executing' | 'completed' | 'aborted';
export const ALL_DIRECTIVE_STATE: readonly DirectiveState[] = ['draft', 'pending', 'approved', 'executing', 'completed', 'aborted'];

export const ENTITY_TRANSITIONS: Readonly<Record<string, Readonly<Record<string, readonly string[]>>>> = {
	reservoir: { normal: ['warning', 'critical'], warning: ['critical', 'restricted', 'normal'], critical: ['restricted', 'warning'], restricted: ['critical'] },
	gateUnit: { open: ['moving', 'locked'], closed: ['moving', 'locked'], moving: ['open', 'closed', 'locked'], locked: ['closed'] },
	operationDirective: { draft: ['pending'], pending: ['approved', 'aborted'], approved: ['executing', 'aborted'], executing: ['completed', 'aborted'], completed: [], aborted: [] },
	executionConfirmation: { pending: ['confirmed', 'failed'], confirmed: [], failed: ['cancelled'], cancelled: [] },
};

export function allowedTransitions(entity: string, status: string): readonly string[] {
	return ENTITY_TRANSITIONS[entity]?.[status] || [];
}

export const ENTITY_CONFIGS: readonly EntityConfig[] = [
  { key: 'reservoir', path: 'reservoirs', label: '库区', statuses: ['normal', 'warning', 'critical', 'restricted'] as const },
  { key: 'gateUnit', path: 'gates', label: '闸门', statuses: ['open', 'closed', 'moving', 'locked'] as const },
  { key: 'operationDirective', path: 'directives', label: '操作指令', statuses: ['draft', 'pending', 'approved', 'executing', 'completed', 'aborted'] as const },
  { key: 'executionConfirmation', path: 'confirmations', label: '执行确认', statuses: ['pending', 'confirmed', 'failed', 'cancelled'] as const }
];
