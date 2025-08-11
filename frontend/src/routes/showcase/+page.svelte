<script lang="ts">
	import SearchBar from '$lib/components/app/SearchBar.svelte';
	import SongCard from '$lib/components/app/SongCard.svelte';
	import PlatformBadge from '$lib/components/app/PlatformBadge.svelte';
	import type { Song, PlatformLink } from '$lib/api/client.js';
	import { PLATFORMS } from '$lib/types/generated.js';

	// Sample song data for showcase
	const sampleSongs: Song[] = [
		{
			id: '1',
			title: 'Bohemian Rhapsody',
			artist: 'Queen',
			album: 'A Night at the Opera',
			duration: 355, // 5:55
			isrc: 'GBUM71505078',
			albumArt: 'https://i.scdn.co/image/ab67616d0000b273ce4f1737bc8a646c8c4bd25a',
			platforms: [
				{
					platform: PLATFORMS.SPOTIFY,
					url: 'https://open.spotify.com/track/4u7EnebtmKWzUH433cf5Qv',
					confidence: 0.95,
					available: true
				},
				{
					platform: PLATFORMS.APPLE_MUSIC,
					url: 'https://music.apple.com/us/album/bohemian-rhapsody/1440650428?i=1440650439',
					confidence: 0.92,
					available: true
				},
				{
					platform: PLATFORMS.TIDAL,
					url: 'https://tidal.com/browse/track/464408',
					confidence: 0.88,
					available: true
				}
			],
			createdAt: new Date('2024-01-15T10:30:00Z'),
			updatedAt: new Date('2024-01-15T10:30:00Z')
		},
		{
			id: '2',
			title: 'Blinding Lights',
			artist: 'The Weeknd',
			album: 'After Hours',
			duration: 200, // 3:20
			isrc: 'USUG11902500',
			albumArt: 'https://i.scdn.co/image/ab67616d0000b273ef6f581fdc8d3171b9a76d69',
			platforms: [
				{
					platform: PLATFORMS.SPOTIFY,
					url: 'https://open.spotify.com/track/0VjIjW4GlULA4LGoDOLVKN',
					confidence: 0.98,
					available: true
				},
				{
					platform: PLATFORMS.APPLE_MUSIC,
					url: 'https://music.apple.com/us/album/blinding-lights/1499378108?i=1499378116',
					confidence: 0.96,
					available: true
				},
				{
					platform: PLATFORMS.TIDAL,
					url: '',
					confidence: 0.45,
					available: false
				}
			],
			createdAt: new Date('2024-01-16T14:22:00Z'),
			updatedAt: new Date('2024-01-16T14:22:00Z')
		},
		{
			id: '3',
			title: 'Hotel California',
			artist: 'Eagles',
			album: 'Hotel California',
			duration: 391, // 6:31
			isrc: 'USEE10001993',
			albumArt: 'https://i.scdn.co/image/ab67616d0000b273c8a11e48c91a982d086afc69',
			platforms: [
				{
					platform: PLATFORMS.SPOTIFY,
					url: 'https://open.spotify.com/track/40riOy7x9W7GXjyGp4pjAv',
					confidence: 0.94,
					available: true
				},
				{
					platform: PLATFORMS.APPLE_MUSIC,
					url: 'https://music.apple.com/us/album/hotel-california/1454269663?i=1454269675',
					confidence: 0.91,
					available: true
				}
			],
			createdAt: new Date('2024-01-17T09:15:00Z'),
			updatedAt: new Date('2024-01-17T09:15:00Z')
		}
	];

	// Sample individual platform links for PlatformBadge showcase
	const samplePlatformLinks: PlatformLink[] = [
		{
			platform: PLATFORMS.SPOTIFY,
			url: 'https://open.spotify.com/track/example',
			confidence: 0.98,
			available: true
		},
		{
			platform: PLATFORMS.APPLE_MUSIC,
			url: 'https://music.apple.com/us/album/example',
			confidence: 0.85,
			available: true
		},
		{
			platform: PLATFORMS.TIDAL,
			url: '',
			confidence: 0.42,
			available: false
		}
	];

	let searchQuery = $state('');
	let isSearching = $state(false);

	function handleSearch(query: string) {
		searchQuery = query;
		isSearching = true;
		
		// Simulate search delay
		setTimeout(() => {
			isSearching = false;
			console.log('Searching for:', query);
		}, 1500);
	}

	function handleClear() {
		searchQuery = '';
		console.log('Search cleared');
	}
</script>

<svelte:head>
	<title>Component Showcase - SongShare</title>
</svelte:head>

<div class="container mx-auto px-4 py-8 max-w-6xl">
	<div class="mb-8">
		<h1 class="text-4xl font-bold mb-2">Component Showcase</h1>
		<p class="text-muted-foreground">
			Preview of all SongShare components with sample data
		</p>
	</div>

	<!-- SearchBar Showcase -->
	<section class="mb-12">
		<h2 class="text-2xl font-semibold mb-4">Search Bar</h2>
		<div class="space-y-4">
			<div class="max-w-md">
				<SearchBar 
					{isSearching}
					onSearch={handleSearch}
					onClear={handleClear}
					placeholder="Search for songs, artists, or albums..."
				/>
			</div>
			<div class="text-sm text-muted-foreground">
				Current query: <code class="bg-muted px-2 py-1 rounded">{searchQuery || 'none'}</code>
			</div>
		</div>
	</section>

	<!-- PlatformBadge Showcase -->
	<section class="mb-12">
		<h2 class="text-2xl font-semibold mb-4">Platform Badges</h2>
		<div class="space-y-6">
			<div>
				<h3 class="text-lg font-medium mb-3">Individual Badges</h3>
				<div class="flex flex-wrap gap-3">
					{#each samplePlatformLinks as link}
						<PlatformBadge {link} />
					{/each}
				</div>
			</div>
			
			<div>
				<h3 class="text-lg font-medium mb-3">Badge States</h3>
				<div class="grid grid-cols-1 md:grid-cols-3 gap-4 text-sm">
					<div class="space-y-2">
						<h4 class="font-medium">High Confidence (Available)</h4>
						<PlatformBadge link={{
							platform: PLATFORMS.SPOTIFY,
							url: 'https://open.spotify.com/track/example',
							confidence: 0.98,
							available: true
						}} />
					</div>
					<div class="space-y-2">
						<h4 class="font-medium">Low Confidence (Available)</h4>
						<PlatformBadge link={{
							platform: PLATFORMS.APPLE_MUSIC,
							url: 'https://music.apple.com/us/album/example',
							confidence: 0.65,
							available: true
						}} />
					</div>
					<div class="space-y-2">
						<h4 class="font-medium">Unavailable</h4>
						<PlatformBadge link={{
							platform: PLATFORMS.TIDAL,
							url: '',
							confidence: 0.30,
							available: false
						}} />
					</div>
				</div>
			</div>
		</div>
	</section>

	<!-- SongCard Showcase -->
	<section class="mb-12">
		<h2 class="text-2xl font-semibold mb-4">Song Cards</h2>
		<div class="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-6">
			{#each sampleSongs as song}
				<SongCard {song} />
			{/each}
		</div>
	</section>

	<!-- Component Integration Example -->
	<section class="mb-12">
		<h2 class="text-2xl font-semibent mb-4">Integration Example</h2>
		<div class="space-y-6">
			<div class="max-w-md">
				<SearchBar 
					isSearching={false}
					onSearch={handleSearch}
					onClear={handleClear}
					placeholder="Try searching for a song..."
				/>
			</div>
			
			{#if searchQuery}
				<div>
					<h3 class="text-lg font-medium mb-3">
						Search Results for "{searchQuery}"
					</h3>
					<div class="grid grid-cols-1 md:grid-cols-2 gap-4">
						{#each sampleSongs.slice(0, 2) as song}
							<SongCard {song} />
						{/each}
					</div>
				</div>
			{:else}
				<div class="text-muted-foreground">
					Enter a search query to see results...
				</div>
			{/if}
		</div>
	</section>
</div>
