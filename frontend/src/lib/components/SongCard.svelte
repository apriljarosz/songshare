<script lang="ts">
	import type { Song } from '$lib/types/generated';
	import PlatformBadge from './PlatformBadge.svelte';

	let { song }: { song: Song } = $props();

	function formatDuration(ms: number): string {
		const seconds = Math.floor(ms / 1000);
		const minutes = Math.floor(seconds / 60);
		const remainingSeconds = seconds % 60;
		return `${minutes}:${remainingSeconds.toString().padStart(2, '0')}`;
	}

	function copyShareLink() {
		const shareUrl = `${window.location.origin}/s/${song.id}`;
		navigator.clipboard.writeText(shareUrl);
	}
</script>

<article class="song-card">
	<img src={song.albumArt} alt="{song.title} album art" class="album-art" />

	<div class="song-info">
		<h3 class="song-title">
			{song.title}
			{#if song.explicit}
				<span class="explicit-badge" title="Explicit">🅴</span>
			{/if}
		</h3>
		<p class="song-artist">{song.artists.join(', ')}</p>
		<p class="song-album">{song.album}</p>

		<div class="metadata">
			{#if song.duration}
				<span class="duration">{formatDuration(song.duration)}</span>
			{/if}
			{#if song.releaseDate}
				<span class="release-date">{new Date(song.releaseDate).getFullYear()}</span>
			{/if}
		</div>

		<div class="platforms">
			{#each song.platforms as platform}
				<PlatformBadge {platform} />
			{/each}
		</div>

		<button onclick={copyShareLink} class="share-button">
			Copy Share Link
		</button>
	</div>
</article>

<style>
	.song-card {
		display: flex;
		gap: 1.5rem;
		padding: 1.5rem;
		background: white;
		border: 1px solid #e2e8f0;
		border-radius: 12px;
		box-shadow: 0 1px 3px rgba(0, 0, 0, 0.1);
		transition: box-shadow 0.2s;
	}

	.song-card:hover {
		box-shadow: 0 4px 6px rgba(0, 0, 0, 0.1);
	}

	.album-art {
		width: 150px;
		height: 150px;
		object-fit: cover;
		border-radius: 8px;
		flex-shrink: 0;
	}

	.song-info {
		flex: 1;
		display: flex;
		flex-direction: column;
		gap: 0.5rem;
	}

	.song-title {
		margin: 0;
		font-size: 1.5rem;
		font-weight: 700;
		color: #1e293b;
		display: flex;
		align-items: center;
		gap: 0.5rem;
	}

	.explicit-badge {
		font-size: 1.25rem;
		opacity: 0.7;
	}

	.song-artist {
		margin: 0;
		font-size: 1.125rem;
		color: #475569;
	}

	.song-album {
		margin: 0;
		font-size: 0.875rem;
		color: #64748b;
	}

	.metadata {
		display: flex;
		gap: 1rem;
		font-size: 0.875rem;
		color: #94a3b8;
	}

	.platforms {
		display: flex;
		flex-wrap: wrap;
		gap: 0.5rem;
		margin-top: 0.5rem;
	}

	.share-button {
		align-self: flex-start;
		padding: 0.5rem 1rem;
		margin-top: auto;
		font-size: 0.875rem;
		font-weight: 600;
		color: #3b82f6;
		background: transparent;
		border: 2px solid #3b82f6;
		border-radius: 6px;
		cursor: pointer;
		transition: all 0.2s;
	}

	.share-button:hover {
		color: white;
		background-color: #3b82f6;
	}

	@media (max-width: 640px) {
		.song-card {
			flex-direction: column;
		}

		.album-art {
			width: 100%;
			height: auto;
			aspect-ratio: 1;
		}
	}
</style>
