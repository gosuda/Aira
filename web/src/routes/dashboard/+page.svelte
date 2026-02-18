<script lang="ts">
	import { goto } from '$app/navigation';
	import { auth, board, adrStore, agentStore } from '$lib/stores';
	import KanbanBoard from '$lib/components/board/KanbanBoard.svelte';
	import ADRTimeline from '$lib/components/adr/ADRTimeline.svelte';
	import AgentMonitor from '$lib/components/agent/AgentMonitor.svelte';
	import { onMount } from 'svelte';

	type TabId = 'board' | 'adrs' | 'agents';
	let activeTab = $state<TabId>('board');

	const tabs: { id: TabId; label: string }[] = [
		{ id: 'board', label: 'Board' },
		{ id: 'adrs', label: 'ADRs' },
		{ id: 'agents', label: 'Agents' }
	];

	$effect(() => {
		if (!auth.loading && !auth.isAuthenticated) {
			goto('/login');
		}
	});

	onMount(() => {
		// Use a demo project ID -- in production this comes from route params
		const projectId = 'demo';
		if (auth.token) {
			board.load(projectId, auth.token);
			adrStore.load(projectId, auth.token);
			agentStore.loadSessions(projectId, auth.token);
		}
	});
</script>

{#if auth.user}
	<div class="min-h-screen bg-gray-50">
		<header class="border-b border-gray-200 bg-white">
			<div class="mx-auto flex max-w-7xl items-center justify-between px-6 py-3">
				<div class="flex items-center gap-6">
					<h1 class="text-lg font-semibold text-gray-900">Aira</h1>
					<nav class="flex gap-1">
						{#each tabs as tab}
							<button
								class="rounded-md px-3 py-1.5 text-sm font-medium transition-colors"
								class:bg-gray-100={activeTab === tab.id}
								class:text-gray-900={activeTab === tab.id}
								class:text-gray-500={activeTab !== tab.id}
								class:hover:text-gray-700={activeTab !== tab.id}
								onclick={() => (activeTab = tab.id)}
							>
								{tab.label}
							</button>
						{/each}
					</nav>
				</div>
				<div class="flex items-center gap-4">
					<span class="text-sm text-gray-600">{auth.user.name}</span>
					<button
						onclick={() => {
							auth.logout();
							goto('/login');
						}}
						class="rounded-md px-3 py-1.5 text-sm text-gray-600 hover:bg-gray-100"
					>
						Sign out
					</button>
				</div>
			</div>
		</header>

		<main class="mx-auto max-w-7xl px-6 py-6">
			{#if activeTab === 'board'}
				<KanbanBoard />
			{:else if activeTab === 'adrs'}
				<ADRTimeline />
			{:else if activeTab === 'agents'}
				<AgentMonitor />
			{/if}
		</main>
	</div>
{:else}
	<div class="flex min-h-screen items-center justify-center">
		<p class="text-gray-500">Loading...</p>
	</div>
{/if}
