<script lang="ts">
	import type { Task } from '$lib/stores/board.svelte';

	let { task }: { task: Task } = $props();

	const priorityColors: Record<number, string> = {
		0: 'bg-gray-200 text-gray-700',
		1: 'bg-blue-100 text-blue-700',
		2: 'bg-amber-100 text-amber-700',
		3: 'bg-red-100 text-red-700'
	};

	function handleDragStart(e: DragEvent) {
		e.dataTransfer?.setData('text/plain', task.id);
		if (e.dataTransfer) {
			e.dataTransfer.effectAllowed = 'move';
		}
	}
</script>

<div
	class="cursor-grab rounded-md border border-gray-200 bg-white p-3 shadow-sm transition-shadow hover:shadow-md active:cursor-grabbing"
	draggable="true"
	ondragstart={handleDragStart}
	role="listitem"
>
	<p class="text-sm font-medium text-gray-900">{task.title}</p>
	{#if task.description}
		<p class="mt-1 line-clamp-2 text-xs text-gray-500">{task.description}</p>
	{/if}
	<div class="mt-2 flex items-center gap-2">
		<span
			class="rounded px-1.5 py-0.5 text-[10px] font-medium {priorityColors[task.priority] ??
				priorityColors[0]}"
		>
			P{task.priority}
		</span>
		{#if task.adrId}
			<span class="rounded bg-purple-100 px-1.5 py-0.5 text-[10px] font-medium text-purple-700"
				>ADR</span
			>
		{/if}
	</div>
</div>
