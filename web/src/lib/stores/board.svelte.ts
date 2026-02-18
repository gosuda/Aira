export interface Task {
	id: string;
	title: string;
	description: string;
	status: 'backlog' | 'in_progress' | 'review' | 'done';
	priority: number;
	assignedTo?: string;
	adrId?: string;
	createdAt: string;
	updatedAt: string;
}

const COLUMNS = ['backlog', 'in_progress', 'review', 'done'] as const;
type ColumnStatus = (typeof COLUMNS)[number];

const COLUMN_LABELS: Record<ColumnStatus, string> = {
	backlog: 'Backlog',
	in_progress: 'In Progress',
	review: 'Review',
	done: 'Done'
};

function createBoardStore() {
	let tasks = $state<Task[]>([]);
	let loading = $state(true);
	let error = $state('');

	async function load(pid: string, token: string) {
		loading = true;
		error = '';
		try {
			const res = await fetch(`/api/v1/projects/${pid}/tasks`, {
				headers: { Authorization: `Bearer ${token}` }
			});
			if (!res.ok) throw new Error(`Failed to load tasks (${res.status})`);
			const data = await res.json();
			tasks = data.tasks ?? [];
		} catch (e) {
			error = e instanceof Error ? e.message : 'Failed to load board';
		} finally {
			loading = false;
		}
	}

	async function moveTask(taskId: string, newStatus: ColumnStatus, token: string) {
		const task = tasks.find((t) => t.id === taskId);
		if (!task || task.status === newStatus) return;

		const oldStatus = task.status;
		// Optimistic update
		task.status = newStatus;

		try {
			const res = await fetch(`/api/v1/tasks/${taskId}/transition`, {
				method: 'POST',
				headers: { 'Content-Type': 'application/json', Authorization: `Bearer ${token}` },
				body: JSON.stringify({ status: newStatus })
			});
			if (!res.ok) throw new Error('Transition failed');
		} catch {
			// Rollback
			task.status = oldStatus;
		}
	}

	function handleWSUpdate(updated: Task) {
		const idx = tasks.findIndex((t) => t.id === updated.id);
		if (idx >= 0) {
			tasks[idx] = updated;
		} else {
			tasks.push(updated);
		}
	}

	return {
		get tasks() {
			return tasks;
		},
		get loading() {
			return loading;
		},
		get error() {
			return error;
		},
		get columns() {
			return COLUMNS;
		},
		get columnLabels() {
			return COLUMN_LABELS;
		},
		byStatus(status: ColumnStatus) {
			return tasks.filter((t) => t.status === status);
		},
		load,
		moveTask,
		handleWSUpdate
	};
}

export const board = createBoardStore();
