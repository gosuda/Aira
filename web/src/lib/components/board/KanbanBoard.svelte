<script lang="ts">
	import { board } from '$lib/stores';
	import { auth } from '$lib/stores';
	import KanbanColumn from './KanbanColumn.svelte';

	function handleDrop(taskId: string, newStatus: string) {
		if (auth.token) {
			board.moveTask(
				taskId,
				newStatus as 'backlog' | 'in_progress' | 'review' | 'done',
				auth.token
			);
		}
	}
</script>

{#if board.loading}
	<div class="flex items-center justify-center py-12">
		<p class="text-sm text-gray-500">Loading board...</p>
	</div>
{:else if board.error}
	<div class="rounded-md bg-red-50 px-4 py-3 text-sm text-red-700">{board.error}</div>
{:else}
	<div class="flex gap-4 overflow-x-auto pb-4">
		{#each board.columns as col}
			<KanbanColumn
				status={col}
				label={board.columnLabels[col]}
				tasks={board.byStatus(col)}
				onDrop={handleDrop}
			/>
		{/each}
	</div>
{/if}
