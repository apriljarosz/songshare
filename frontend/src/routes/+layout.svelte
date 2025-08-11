<script lang="ts">
	import '../app.css';
	import favicon from '$lib/assets/favicon.svg';
	import { Button } from '$lib/components/ui/button';

	let { children } = $props();
</script>

<svelte:head>
	<link rel="icon" href={favicon} />
	<title>SongShare - Share Music Across Platforms</title>
	<meta name="description" content="Resolve and share songs across Spotify, Apple Music, and Tidal" />
</svelte:head>

<div class="min-h-screen bg-background">
	<!-- Header -->
	<header class="border-b border-border bg-card">
		<div class="container mx-auto px-4 py-4">
			<nav class="flex items-center justify-between">
				<div class="flex items-center space-x-4">
					<a href="/" class="text-2xl font-bold text-foreground hover:text-primary transition-colors">
						SongShare
					</a>
				</div>
				
				<div class="flex items-center space-x-4">
					<a href="/search" class="text-muted-foreground hover:text-foreground transition-colors">
						Search
					</a>
					<a href="/admin" class="text-muted-foreground hover:text-foreground transition-colors">
						Admin
					</a>
					<Button variant="outline" size="sm">
						About
					</Button>
				</div>
			</nav>
		</div>
	</header>

	<!-- Main Content -->
	<main class="flex-1">
		{@render children?.()}
	</main>

	<!-- Footer -->
	<footer class="border-t border-border bg-muted/50 mt-auto">
		<div class="container mx-auto px-4 py-6">
			<div class="flex items-center justify-between text-sm text-muted-foreground">
				<p>&copy; 2025 SongShare. Share music across platforms.</p>
				<div class="flex items-center space-x-4">
					<span>Supports:</span>
					<span class="inline-flex items-center px-2 py-1 rounded-full bg-green-100 text-green-800 text-xs">
						Spotify
					</span>
					<span class="inline-flex items-center px-2 py-1 rounded-full bg-gray-100 text-gray-800 text-xs">
						Apple Music
					</span>
					<span class="inline-flex items-center px-2 py-1 rounded-full bg-blue-100 text-blue-800 text-xs">
						Tidal
					</span>
				</div>
			</div>
		</div>
	</footer>
</div>
