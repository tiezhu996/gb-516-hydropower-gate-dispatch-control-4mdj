
export function formatDate(value: string): string {
  return value ? new Intl.DateTimeFormat('zh-CN', { dateStyle: 'medium', timeStyle: 'short' }).format(new Date(value)) : '-';
}

const STATUS_LABELS: Readonly<Record<string, string>> = {
  normal: '正常',
  warning: '预警',
  critical: '严重',
  restricted: '受限',
  open: '开启',
  closed: '关闭',
  moving: '动作中',
  locked: '闭锁',
  draft: '草稿',
  pending: '待处理',
  approved: '已批准',
  executing: '执行中',
  completed: '已完成',
  aborted: '已中止',
  confirmed: '已确认',
  failed: '失败',
  cancelled: '已取消',
};

const RISK_LABELS: Readonly<Record<string, string>> = {
  low: '低',
  medium: '中',
  high: '高',
  critical: '极高',
};

export function statusLabel(status: string): string {
  return STATUS_LABELS[status] || status.replaceAll('_', ' ');
}

export function riskLabel(risk: string): string {
  return RISK_LABELS[risk] || risk;
}

export function nextStatus(current: string, statuses: readonly string[]): string | null {
  const index = statuses.indexOf(current);
  return index >= 0 && index < statuses.length - 1 ? statuses[index + 1] : null;
}
export function statusTone(status: string): 'success' | 'warning' | 'danger' | 'neutral' {
  if (/approved|accepted|released|completed|signed|closed|pass|ready|online|cleared|succeeded/.test(status)) return 'success';
  if (/failed|rejected|critical|scrap|discard|revoked|urgent/.test(status)) return 'danger';
  if (/hold|warning|review|pending|restricted|limited|quarantine/.test(status)) return 'warning';
  return 'neutral';
}
