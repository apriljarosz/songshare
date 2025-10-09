<script lang="ts">
	let { isLoading = $bindable(false), onsearch }: { isLoading?: boolean; onsearch?: (query: string) => void } = $props();

	let query = $state('');

	function handleSubmit(event: Event) {
		event.preventDefault();
		if (query.trim() && onsearch) {
			onsearch(query.trim());
		}
	}
</script>

<form onsubmit={handleSubmit} class="search-bar">
	<input
		type="text"
		bind:value={query}
		placeholder="Search for a song or paste a platform URL..."
		disabled={isLoading}
		class="search-input"
	/>
	<button type="submit" disabled={isLoading || !query.trim()} class="search-button">
		{isLoading ? 'Searching...' : 'Search'}
	</button>
</form>

<style>
	.search-bar {
		display: flex;
		gap: 0.5rem;
		width: 100%;
		max-width: 600px;
	}

	.search-input {
		flex: 1;
		padding: 0.75rem 1rem;
		font-size: 1rem;
		border: 2px solid #e2e8f0;
		border-radius: 8px;
		outline: none;
		transition: border-color 0.2s;
	}

	.search-input:focus {
		border-color: #3b82f6;
	}

	.search-input:disabled {
		background-color: #f1f5f9;
		cursor: not-allowed;
	}

	.search-button {
		padding: 0.75rem 1.5rem;
		font-size: 1rem;
		font-weight: 600;
		color: white;
		background-color: #3b82f6;
		border: none;
		border-radius: 8px;
		cursor: pointer;
		transition: background-color 0.2s;
	}

	.search-button:hover:not(:disabled) {
		background-color: #2563eb;
	}

	.search-button:disabled {
		background-color: #94a3b8;
		cursor: not-allowed;
	}
</style>
