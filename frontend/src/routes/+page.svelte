<script lang="ts">
	import { onMount } from 'svelte';
	import SearchBar from '$lib/components/SearchBar.svelte';
	import SongCard from '$lib/components/SongCard.svelte';
	import apiClient from '$lib/api/client';
	import type { Song } from '$lib/types/generated';

	let songs = $state<Song[]>([]);
	let isLoading = $state(false);
	let isLoadingMore = $state(false);
	let error = $state<string | null>(null);
	let hasSearched = $state(false);
	let currentQuery = $state<string>('');
	let currentOffset = $state(0);
	let hasMore = $state(true);
	const LIMIT = 10;

	async function handleSearch(query: string) {
		console.log('Search started:', query);
		isLoading = true;
		error = null;
		hasSearched = true;
		currentQuery = query;
		currentOffset = 0;
		hasMore = true;

		try {
			// Check if query is a URL
			if (query.startsWith('http://') || query.startsWith('https://')) {
				console.log('Resolving URL...');
				const song = await apiClient.resolve(query);
				console.log('Resolved song:', song);
				songs = [song];
				hasMore = false;
			} else {
				console.log('Searching...');
				const results = await apiClient.search(query, 0, LIMIT);
				console.log('Search results:', results);
				songs = results;
				currentOffset = results.length;
				hasMore = results.length === LIMIT;
			}
		} catch (err) {
			console.error('Search error:', err);
			error = err instanceof Error ? err.message : 'An error occurred';
			songs = [];
			hasMore = false;
		} finally {
			isLoading = false;
			console.log('Search completed. Songs:', songs.length);
		}
	}

	async function loadMore() {
		if (isLoadingMore || !hasMore || !currentQuery || currentQuery.startsWith('http')) {
			return;
		}

		isLoadingMore = true;
		try {
			const results = await apiClient.search(currentQuery, currentOffset, LIMIT);
			songs = [...songs, ...results];
			currentOffset += results.length;
			hasMore = results.length === LIMIT;
		} catch (err) {
			console.error('Load more error:', err);
		} finally {
			isLoadingMore = false;
		}
	}

	function handleScroll() {
		const scrollPosition = window.innerHeight + window.scrollY;
		const pageHeight = document.documentElement.scrollHeight;

		// Load more when user is within 500px of the bottom
		if (scrollPosition >= pageHeight - 500 && !isLoadingMore && hasMore) {
			loadMore();
		}
	}

	onMount(() => {
		window.addEventListener('scroll', handleScroll);
		return () => {
			window.removeEventListener('scroll', handleScroll);
		};
	});
</script>

<svelte:head>
	<title>SongShare - Universal Music Search</title>
	<meta name="description" content="Search for songs across all platforms" />
</svelte:head>

<main>
	<header class="header">
		<h1 class="title">SongShare</h1>
		<p class="subtitle">Find your music everywhere</p>
	</header>

	<div class="search-container">
		<SearchBar bind:isLoading onsearch={handleSearch} />
	</div>

	{#if error}
		<div class="error">
			<p>{error}</p>
		</div>
	{/if}

	{#if isLoading}
		<div class="loading">
			<p>Searching...</p>
		</div>
	{:else if hasSearched && songs.length === 0 && !error}
		<div class="no-results">
			<p>No songs found. Try a different search.</p>
		</div>
	{:else if songs.length > 0}
		<div class="results">
			{#each songs as song, index (`${song.id}-${index}`)}
				<SongCard {song} />
			{/each}
		</div>
		{#if isLoadingMore}
			<div class="loading-more">
				<p>Loading more...</p>
			</div>
		{/if}
	{:else}
		<div class="welcome">
			<p>Search for a song by name or paste a Spotify/Apple Music URL to get started</p>
		</div>
	{/if}
</main>

<style>
	:global(body) {
		margin: 0;
		padding: 0;
		font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, Oxygen, Ubuntu, Cantarell, sans-serif;
		background-color: #f8fafc;
		color: #1e293b;
	}

	main {
		max-width: 1200px;
		margin: 0 auto;
		padding: 2rem;
		min-height: 100vh;
	}

	.header {
		text-align: center;
		margin-bottom: 3rem;
	}

	.title {
		font-size: 3rem;
		font-weight: 800;
		margin: 0;
		background: linear-gradient(135deg, #3b82f6 0%, #8b5cf6 100%);
		-webkit-background-clip: text;
		-webkit-text-fill-color: transparent;
		background-clip: text;
	}

	.subtitle {
		font-size: 1.25rem;
		color: #64748b;
		margin: 0.5rem 0 0;
	}

	.search-container {
		display: flex;
		justify-content: center;
		margin-bottom: 2rem;
	}

	.results {
		display: flex;
		flex-direction: column;
		gap: 1.5rem;
	}

	.error,
	.loading,
	.loading-more,
	.no-results,
	.welcome {
		text-align: center;
		padding: 2rem;
		font-size: 1.125rem;
		color: #64748b;
	}

	.loading-more {
		padding: 1rem;
		font-size: 1rem;
	}

	.error {
		color: #dc2626;
		background-color: #fee2e2;
		border-radius: 8px;
	}

	@media (max-width: 640px) {
		main {
			padding: 1rem;
		}

		.title {
			font-size: 2rem;
		}

		.subtitle {
			font-size: 1rem;
		}
	}
</style>