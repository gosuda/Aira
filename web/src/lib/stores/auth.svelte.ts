// Auth store with Svelte 5 runes
interface User {
	id: string;
	email: string;
	name: string;
	role: string;
	tenantId: string;
}

interface AuthState {
	token: string | null;
	user: User | null;
	loading: boolean;
}

function createAuthStore() {
	let state = $state<AuthState>({
		token: null,
		user: null,
		loading: true
	});

	function init() {
		if (typeof window === 'undefined') {
			state.loading = false;
			return;
		}
		const saved = localStorage.getItem('aira_token');
		if (saved) {
			state.token = saved;
			// Decode JWT payload for user info (no verification needed client-side)
			try {
				const payload = JSON.parse(atob(saved.split('.')[1]));
				state.user = {
					id: payload.sub,
					email: payload.email,
					name: payload.name,
					role: payload.role,
					tenantId: payload.tenant_id
				};
			} catch {
				localStorage.removeItem('aira_token');
				state.token = null;
			}
		}
		state.loading = false;
	}

	function login(token: string) {
		localStorage.setItem('aira_token', token);
		state.token = token;
		try {
			const payload = JSON.parse(atob(token.split('.')[1]));
			state.user = {
				id: payload.sub,
				email: payload.email,
				name: payload.name,
				role: payload.role,
				tenantId: payload.tenant_id
			};
		} catch {
			// Invalid token
		}
	}

	function logout() {
		localStorage.removeItem('aira_token');
		state.token = null;
		state.user = null;
	}

	return {
		get token() {
			return state.token;
		},
		get user() {
			return state.user;
		},
		get loading() {
			return state.loading;
		},
		get isAuthenticated() {
			return state.token !== null;
		},
		init,
		login,
		logout
	};
}

export const auth = createAuthStore();
