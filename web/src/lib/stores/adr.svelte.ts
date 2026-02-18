export interface ADR {
	id: string;
	sequence: number;
	title: string;
	status: 'draft' | 'proposed' | 'accepted' | 'rejected' | 'deprecated';
	context: string;
	decision: string;
	consequences: { good: string[]; bad: string[]; neutral: string[] };
	createdAt: string;
	updatedAt: string;
}

function createADRStore() {
	let adrs = $state<ADR[]>([]);
	let loading = $state(true);
	let error = $state('');
	let selected = $state<ADR | null>(null);

	async function load(projectId: string, token: string) {
		loading = true;
		error = '';
		try {
			const res = await fetch(`/api/v1/projects/${projectId}/adrs`, {
				headers: { Authorization: `Bearer ${token}` }
			});
			if (!res.ok) throw new Error(`Failed to load ADRs (${res.status})`);
			const data = await res.json();
			adrs = data.adrs ?? [];
		} catch (e) {
			error = e instanceof Error ? e.message : 'Failed to load ADRs';
		} finally {
			loading = false;
		}
	}

	function select(adr: ADR | null) {
		selected = adr;
	}

	return {
		get adrs() {
			return adrs;
		},
		get loading() {
			return loading;
		},
		get error() {
			return error;
		},
		get selected() {
			return selected;
		},
		load,
		select
	};
}

export const adrStore = createADRStore();
