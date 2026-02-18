export interface AgentSession {
	id: string;
	taskId?: string;
	agentType: string;
	status: 'pending' | 'running' | 'waiting_hitl' | 'completed' | 'failed' | 'cancelled';
	branchName?: string;
	startedAt?: string;
	completedAt?: string;
	error?: string;
	createdAt: string;
}

export interface AgentLog {
	timestamp: string;
	level: string;
	message: string;
}

function createAgentStore() {
	let sessions = $state<AgentSession[]>([]);
	let logs = $state<AgentLog[]>([]);
	let activeSid = $state<string | null>(null);
	let loading = $state(true);
	let error = $state('');

	async function loadSessions(projectId: string, token: string) {
		loading = true;
		error = '';
		try {
			const res = await fetch(`/api/v1/projects/${projectId}/agents`, {
				headers: { Authorization: `Bearer ${token}` }
			});
			if (!res.ok) throw new Error(`Failed to load sessions (${res.status})`);
			const data = await res.json();
			sessions = data.sessions ?? [];
		} catch (e) {
			error = e instanceof Error ? e.message : 'Failed to load sessions';
		} finally {
			loading = false;
		}
	}

	function selectSession(sessionId: string | null) {
		activeSid = sessionId;
		logs = [];
	}

	function appendLog(log: AgentLog) {
		logs.push(log);
		// Keep last 500 lines
		if (logs.length > 500) {
			logs = logs.slice(-500);
		}
	}

	function updateSession(updated: AgentSession) {
		const idx = sessions.findIndex((s) => s.id === updated.id);
		if (idx >= 0) {
			sessions[idx] = updated;
		} else {
			sessions.unshift(updated);
		}
	}

	return {
		get sessions() {
			return sessions;
		},
		get logs() {
			return logs;
		},
		get activeSessionId() {
			return activeSid;
		},
		get activeSession() {
			return sessions.find((s) => s.id === activeSid) ?? null;
		},
		get loading() {
			return loading;
		},
		get error() {
			return error;
		},
		loadSessions,
		selectSession,
		appendLog,
		updateSession
	};
}

export const agentStore = createAgentStore();
