<script lang="ts">
	import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '$lib/components/ui/card';
	import PlatformBadge from './PlatformBadge.svelte';
	import { ExternalLink, Music, Clock } from 'lucide-svelte';
	import type { Song, Platform } from '$lib/types/generated';

	interface Props {
		song: Song;
		class?: string;
	}

	let { song, class: className = '' }: Props = $props();

	// Format duration from seconds to MM:SS
	function formatDuration(seconds?: number): string {
		if (!seconds) return '';
		const minutes = Math.floor(seconds / 60);
		const remainingSeconds = seconds % 60;
		return `${minutes}:${remainingSeconds.toString().padStart(2, '0')}`;
	}

	// Get the best platform link (highest confidence, available)
	function getBestPlatformLink() {
		if (!song.platforms || song.platforms.length === 0) return null;
		return song.platforms
			.filter(link => link.available)
			.sort((a, b) => b.confidence - a.confidence)[0];
	}

	const bestLink = getBestPlatformLink();
	const duration = formatDuration(song.duration);
</script>

<Card class="hover:shadow-md transition-shadow cursor-pointer {className}">
	<CardHeader class="pb-3">
		<div class="flex items-start justify-between">
			<div class="flex-1 min-w-0">
				<CardTitle class="text-lg font-semibold truncate">{song.title}</CardTitle>
				<CardDescription class="text-sm text-muted-foreground truncate">
					{song.artist}
					{#if song.album}
						• {song.album}
					{/if}
				</CardDescription>
			</div>
			
			{#if song.albumArt}
				<img 
					src={song.albumArt} 
					alt="{song.title} album art"
					class="w-12 h-12 rounded-md object-cover ml-3 flex-shrink-0"
				/>
			{:else}
				<div class="w-12 h-12 rounded-md bg-muted flex items-center justify-center ml-3 flex-shrink-0">
					<Music class="w-6 h-6 text-muted-foreground" />
				</div>
			{/if}
		</div>
	</CardHeader>
	
	<CardContent class="pt-0">
		<div class="space-y-3">
			<!-- Platform badges -->
			<div class="flex flex-wrap gap-1">
				{#each song.platforms as link}
					<PlatformBadge {link} />
				{/each}
			</div>
			
			<!-- Song metadata -->
			<div class="flex items-center justify-between text-xs text-muted-foreground">
				<div class="flex items-center gap-3">
					{#if duration}
						<div class="flex items-center gap-1">
							<Clock class="w-3 h-3" />
							{duration}
						</div>
					{/if}
					
					{#if song.isrc}
						<span class="font-mono text-xs">ISRC: {song.isrc}</span>
					{/if}
				</div>
				
				{#if bestLink}
					<a 
						href={bestLink.url} 
						target="_blank" 
						rel="noopener noreferrer"
						class="flex items-center gap-1 hover:text-foreground transition-colors"
						onclick={(e) => e.stopPropagation()}
					>
						<ExternalLink class="w-3 h-3" />
						Listen
					</a>
				{/if}
			</div>
		</div>
	</CardContent>
</Card>