<script lang="ts">
	import { adrStore } from '$lib/stores';
	import type { ADR } from '$lib/stores/adr.svelte';

	const statusBadge: Record<string, string> = {
		draft: 'bg-gray-100 text-gray-700',
		proposed: 'bg-blue-100 text-blue-700',
		accepted: 'bg-green-100 text-green-700',
		rejected: 'bg-red-100 text-red-700',
		deprecated: 'bg-amber-100 text-amber-700'
	};

	function handleSelect(adr: ADR) {
		adrStore.select(adrStore.selected?.id === adr.id ? null : adr);
	}
</script>

{#if adrStore.loading}
	<p class="py-8 text-center text-sm text-gray-500">Loading ADRs...</p>
{:else if adrStore.error}
	<div class="rounded-md bg-red-50 px-4 py-3 text-sm text-red-700">{adrStore.error}</div>
{:else if adrStore.adrs.length === 0}
	<p class="py-8 text-center text-sm text-gray-400">No architectural decisions recorded yet.</p>
{:else}
	<div class="flex gap-6">
		<!-- Timeline list -->
		<div class="w-80 shrink-0 space-y-2">
			{#each adrStore.adrs as adr (adr.id)}
				<button
					class="w-full rounded-lg border p-3 text-left transition-colors"
					class:border-gray-900={adrStore.selected?.id === adr.id}
					class:bg-gray-50={adrStore.selected?.id === adr.id}
					class:border-gray-200={adrStore.selected?.id !== adr.id}
					class:hover:bg-gray-50={adrStore.selected?.id !== adr.id}
					onclick={() => handleSelect(adr)}
				>
					<div class="flex items-center gap-2">
						<span class="font-mono text-xs text-gray-400">#{adr.sequence}</span>
						<span
							class="rounded px-1.5 py-0.5 text-[10px] font-medium {statusBadge[adr.status] ??
								statusBadge.draft}"
						>
							{adr.status}
						</span>
					</div>
					<p class="mt-1 text-sm font-medium text-gray-900">{adr.title}</p>
					<p class="mt-0.5 text-xs text-gray-500">
						{new Date(adr.createdAt).toLocaleDateString()}
					</p>
				</button>
			{/each}
		</div>

		<!-- Detail panel -->
		{#if adrStore.selected}
			<div class="flex-1 rounded-lg border border-gray-200 bg-white p-6">
				<div class="flex items-center gap-3">
					<span class="font-mono text-sm text-gray-400"
						>ADR-{String(adrStore.selected.sequence).padStart(4, '0')}</span
					>
					<span
						class="rounded px-2 py-0.5 text-xs font-medium {statusBadge[
							adrStore.selected.status
						] ?? statusBadge.draft}"
					>
						{adrStore.selected.status}
					</span>
				</div>
				<h2 class="mt-2 text-lg font-semibold text-gray-900">{adrStore.selected.title}</h2>

				<section class="mt-4">
					<h3 class="text-sm font-semibold text-gray-700">Context</h3>
					<p class="mt-1 whitespace-pre-wrap text-sm text-gray-600">
						{adrStore.selected.context}
					</p>
				</section>

				<section class="mt-4">
					<h3 class="text-sm font-semibold text-gray-700">Decision</h3>
					<p class="mt-1 whitespace-pre-wrap text-sm text-gray-600">
						{adrStore.selected.decision}
					</p>
				</section>

				{#if adrStore.selected.consequences}
					<section class="mt-4">
						<h3 class="text-sm font-semibold text-gray-700">Consequences</h3>
						<div class="mt-2 space-y-2">
							{#if adrStore.selected.consequences.good?.length}
								<div>
									<span class="text-xs font-medium text-green-700">Good:</span>
									<ul class="ml-4 list-disc text-sm text-gray-600">
										{#each adrStore.selected.consequences.good as item}
											<li>{item}</li>
										{/each}
									</ul>
								</div>
							{/if}
							{#if adrStore.selected.consequences.bad?.length}
								<div>
									<span class="text-xs font-medium text-red-700">Bad:</span>
									<ul class="ml-4 list-disc text-sm text-gray-600">
										{#each adrStore.selected.consequences.bad as item}
											<li>{item}</li>
										{/each}
									</ul>
								</div>
							{/if}
							{#if adrStore.selected.consequences.neutral?.length}
								<div>
									<span class="text-xs font-medium text-gray-500">Neutral:</span>
									<ul class="ml-4 list-disc text-sm text-gray-600">
										{#each adrStore.selected.consequences.neutral as item}
											<li>{item}</li>
										{/each}
									</ul>
								</div>
							{/if}
						</div>
					</section>
				{/if}
			</div>
		{:else}
			<div
				class="flex flex-1 items-center justify-center rounded-lg border border-dashed border-gray-300 bg-gray-50"
			>
				<p class="text-sm text-gray-400">Select an ADR to view details</p>
			</div>
		{/if}
	</div>
{/if}
