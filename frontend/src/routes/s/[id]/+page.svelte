<script lang="ts">
	import SongCard from '$lib/components/SongCard.svelte';
	import type { PageData } from './$types';

	let { data }: { data: PageData } = $props();
</script>

<svelte:head>
	{#if data.song}
		<title>{data.song.title} by {data.song.artists.join(', ')} - SongShare</title>
		<meta name="description" content="Listen to {data.song.title} on your favorite platform" />
	{:else}
		<title>Song not found - SongShare</title>
	{/if}
</svelte:head>

<main>
	<header class="header">
		<a href="/" class="logo">SongShare</a>
	</header>

	{#if data.error}
		<div class="error-container">
			<h2>Oops! Something went wrong</h2>
			<p>{data.error}</p>
			<a href="/" class="home-link">Go back home</a>
		</div>
	{:else if data.song}
		<div class="song-container">
			<SongCard song={data.song} />
		</div>
	{/if}
</main>

<style>
	:global(body) {
		margin: 0;
		padding: 0;
		font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, Oxygen, Ubuntu, Cantarell,
			sans-serif;
		background-color: #f8fafc;
		color: #1e293b;
	}

	main {
		max-width: 800px;
		margin: 0 auto;
		padding: 2rem;
		min-height: 100vh;
	}

	.header {
		text-align: center;
		margin-bottom: 3rem;
	}

	.logo {
		font-size: 2rem;
		font-weight: 800;
		text-decoration: none;
		background: linear-gradient(135deg, #3b82f6 0%, #8b5cf6 100%);
		-webkit-background-clip: text;
		-webkit-text-fill-color: transparent;
		background-clip: text;
	}

	.song-container {
		display: flex;
		justify-content: center;
	}

	.error-container {
		text-align: center;
		padding: 3rem 1rem;
	}

	.error-container h2 {
		font-size: 2rem;
		margin-bottom: 1rem;
		color: #dc2626;
	}

	.error-container p {
		font-size: 1.125rem;
		color: #64748b;
		margin-bottom: 2rem;
	}

	.home-link {
		display: inline-block;
		padding: 0.75rem 1.5rem;
		font-size: 1rem;
		font-weight: 600;
		color: white;
		background-color: #3b82f6;
		border-radius: 8px;
		text-decoration: none;
		transition: background-color 0.2s;
	}

	.home-link:hover {
		background-color: #2563eb;
	}

	@media (max-width: 640px) {
		main {
			padding: 1rem;
		}

		.logo {
			font-size: 1.5rem;
		}
	}
</style>
