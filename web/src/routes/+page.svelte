<script lang="ts">
	import { goto } from '$app/navigation';
	import { auth } from '$lib/stores';

	$effect(() => {
		if (!auth.loading) {
			if (auth.isAuthenticated) {
				goto('/dashboard');
			} else {
				goto('/login');
			}
		}
	});
</script>

<div class="flex items-center justify-center min-h-screen">
	<p class="text-gray-500">Loading...</p>
</div>
