<script lang="ts">
	import type { PlatformLink } from '$lib/types/generated';

	let { platform }: { platform: PlatformLink } = $props();

	const platformNames: Record<string, string> = {
		spotify: 'Spotify',
		apple_music: 'Apple Music',
		youtube_music: 'YouTube Music',
		tidal: 'Tidal',
		deezer: 'Deezer',
	};

	const platformColors: Record<string, string> = {
		spotify: '#1DB954',
		apple_music: '#FC3C44',
		youtube_music: '#FF0000',
		tidal: '#000000',
		deezer: '#FF0092',
	};

	const displayName = platformNames[platform.platform] || platform.platform;
	const color = platformColors[platform.platform] || '#6B7280';
</script>

<a href={platform.url} target="_blank" rel="noopener noreferrer" class="badge" style="--color: {color}">
	{displayName}
</a>

<style>
	.badge {
		display: inline-block;
		padding: 0.5rem 1rem;
		font-size: 0.875rem;
		font-weight: 600;
		color: white;
		background-color: var(--color);
		border-radius: 6px;
		text-decoration: none;
		transition: opacity 0.2s, transform 0.2s;
	}

	.badge:hover {
		opacity: 0.9;
		transform: translateY(-2px);
	}
</style>
