<script lang="ts">
	import { agentStore } from '$lib/stores';
	import type { AgentSession } from '$lib/stores/agent.svelte';

	const statusColors: Record<string, string> = {
		pending: 'bg-gray-100 text-gray-700',
		running: 'bg-blue-100 text-blue-700',
		waiting_hitl: 'bg-amber-100 text-amber-700',
		completed: 'bg-green-100 text-green-700',
		failed: 'bg-red-100 text-red-700',
		cancelled: 'bg-gray-200 text-gray-500'
	};

	function selectSession(session: AgentSession) {
		agentStore.selectSession(agentStore.activeSessionId === session.id ? null : session.id);
	}
</script>

{#if agentStore.loading}
	<p class="py-8 text-center text-sm text-gray-500">Loading agent sessions...</p>
{:else if agentStore.error}
	<div class="rounded-md bg-red-50 px-4 py-3 text-sm text-red-700">{agentStore.error}</div>
{:else if agentStore.sessions.length === 0}
	<p class="py-8 text-center text-sm text-gray-400">No agent sessions yet.</p>
{:else}
	<div class="flex gap-6">
		<!-- Session list -->
		<div class="w-72 shrink-0 space-y-2">
			{#each agentStore.sessions as session (session.id)}
				<button
					class="w-full rounded-lg border p-3 text-left transition-colors"
					class:border-gray-900={agentStore.activeSessionId === session.id}
					class:bg-gray-50={agentStore.activeSessionId === session.id}
					class:border-gray-200={agentStore.activeSessionId !== session.id}
					onclick={() => selectSession(session)}
				>
					<div class="flex items-center gap-2">
						<span
							class="rounded px-1.5 py-0.5 text-[10px] font-medium {statusColors[
								session.status
							] ?? statusColors.pending}"
						>
							{session.status.replace('_', ' ')}
						</span>
						<span class="text-xs text-gray-400">{session.agentType}</span>
					</div>
					<p class="mt-1 truncate text-sm text-gray-700">{session.id.slice(0, 8)}...</p>
					{#if session.branchName}
						<p class="mt-0.5 truncate font-mono text-xs text-gray-400">{session.branchName}</p>
					{/if}
				</button>
			{/each}
		</div>

		<!-- Log stream -->
		<div class="flex flex-1 flex-col rounded-lg border border-gray-200 bg-gray-900">
			{#if agentStore.activeSession}
				<div class="flex items-center justify-between border-b border-gray-700 px-4 py-2">
					<span class="text-sm text-gray-300">
						Agent: {agentStore.activeSession.agentType} — {agentStore.activeSession.status}
					</span>
					{#if agentStore.activeSession.error}
						<span class="text-xs text-red-400">{agentStore.activeSession.error}</span>
					{/if}
				</div>
				<div class="flex-1 overflow-y-auto p-4 font-mono text-xs">
					{#if agentStore.logs.length === 0}
						<p class="text-gray-500">Waiting for output...</p>
					{:else}
						{#each agentStore.logs as log}
							<div class="py-0.5">
								<span class="text-gray-500"
									>{new Date(log.timestamp).toLocaleTimeString()}</span
								>
								<span
									class:text-red-400={log.level === 'error'}
									class:text-amber-400={log.level === 'warn'}
									class:text-gray-300={log.level !== 'error' && log.level !== 'warn'}
								>
									{log.message}
								</span>
							</div>
						{/each}
					{/if}
				</div>
			{:else}
				<div class="flex flex-1 items-center justify-center">
					<p class="text-sm text-gray-500">Select a session to view logs</p>
				</div>
			{/if}
		</div>
	</div>
{/if}
