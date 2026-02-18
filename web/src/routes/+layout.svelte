<script lang="ts">
	import '../app.css';
	import { auth } from '$lib/stores';
	import { onMount } from 'svelte';

	let { children } = $props();

	onMount(() => {
		auth.init();
	});
</script>

{@render children()}
