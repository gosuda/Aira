<script lang="ts">
	import type { Task } from '$lib/stores/board.svelte';
	import TaskCard from './TaskCard.svelte';

	let {
		status,
		label,
		tasks,
		onDrop
	}: {
		status: string;
		label: string;
		tasks: Task[];
		onDrop: (taskId: string, status: string) => void;
	} = $props();

	let dragOver = $state(false);

	function handleDragOver(e: DragEvent) {
		e.preventDefault();
		dragOver = true;
	}

	function handleDragLeave() {
		dragOver = false;
	}

	function handleDrop(e: DragEvent) {
		e.preventDefault();
		dragOver = false;
		const taskId = e.dataTransfer?.getData('text/plain');
		if (taskId) {
			onDrop(taskId, status);
		}
	}
</script>

<div
	class="flex w-72 shrink-0 flex-col rounded-lg bg-gray-100 p-3"
	class:ring-2={dragOver}
	class:ring-gray-400={dragOver}
	role="list"
	aria-label="{label} column"
	ondragover={handleDragOver}
	ondragleave={handleDragLeave}
	ondrop={handleDrop}
>
	<div class="mb-3 flex items-center justify-between">
		<h3 class="text-sm font-semibold text-gray-700">{label}</h3>
		<span class="rounded-full bg-gray-200 px-2 py-0.5 text-xs text-gray-600">{tasks.length}</span>
	</div>

	<div class="flex flex-col gap-2">
		{#each tasks as task (task.id)}
			<TaskCard {task} />
		{/each}

		{#if tasks.length === 0}
			<p class="py-4 text-center text-xs text-gray-400">No tasks</p>
		{/if}
	</div>
</div>
