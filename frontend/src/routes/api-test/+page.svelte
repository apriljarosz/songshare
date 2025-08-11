<script lang="ts">
	import { apiClient, ApiError } from '$lib/api/client.js';
	import type { SearchSongsResponse, ResolveSongResponse } from '$lib/api/client.js';
	import { Button } from '$lib/components/ui/button';
	import { Input } from '$lib/components/ui/input';
	import { Card, CardContent, CardHeader, CardTitle } from '$lib/components/ui/card';

	let searchQuery = $state('Bohemian Rhapsody Queen');
	let resolveUrl = $state('https://open.spotify.com/track/4u7EnebtmKWzUH433cf5Qv');
	
	let searchResults = $state<SearchSongsResponse | null>(null);
	let resolveResults = $state<ResolveSongResponse | null>(null);
	let searchError = $state<string | null>(null);
	let resolveError = $state<string | null>(null);
	let searchLoading = $state(false);
	let resolveLoading = $state(false);

	async function testSearch() {
		searchLoading = true;
		searchError = null;
		searchResults = null;

		try {
			const response = await apiClient.searchSongs({
				query: searchQuery,
				limit: 5
			});
			searchResults = response;
			console.log('Search results:', response);
		} catch (error) {
			console.error('Search error:', error);
			if (error instanceof ApiError) {
				searchError = `${error.status}: ${error.message}`;
			} else {
				searchError = 'Unknown error occurred';
			}
		} finally {
			searchLoading = false;
		}
	}

	async function testResolve() {
		resolveLoading = true;
		resolveError = null;
		resolveResults = null;

		try {
			const response = await apiClient.resolveSong({
				url: resolveUrl
			});
			resolveResults = response;
			console.log('Resolve results:', response);
		} catch (error) {
			console.error('Resolve error:', error);
			if (error instanceof ApiError) {
				resolveError = `${error.status}: ${error.message}`;
			} else {
				resolveError = 'Unknown error occurred';
			}
		} finally {
			resolveLoading = false;
		}
	}
</script>

<svelte:head>
	<title>API Client Test - SongShare</title>
</svelte:head>

<div class="container mx-auto px-4 py-8 max-w-4xl">
	<h1 class="text-3xl font-bold mb-6">API Client Test</h1>
	<p class="text-muted-foreground mb-8">
		Test the type-safe API client with your backend endpoints
	</p>

	<div class="grid grid-cols-1 lg:grid-cols-2 gap-6">
		<!-- Search Test -->
		<Card>
			<CardHeader>
				<CardTitle>Search Songs</CardTitle>
			</CardHeader>
			<CardContent class="space-y-4">
				<div class="space-y-2">
					<label for="search-query" class="text-sm font-medium">Search Query</label>
					<Input
						id="search-query"
						bind:value={searchQuery}
						placeholder="Enter song, artist, or album..."
					/>
				</div>
				
				<Button 
					onclick={testSearch} 
					disabled={searchLoading || !searchQuery.trim()}
					class="w-full"
				>
					{searchLoading ? 'Searching...' : 'Test Search'}
				</Button>

				{#if searchError}
					<div class="p-3 bg-red-50 border border-red-200 rounded-md">
						<p class="text-red-800 text-sm font-medium">Error:</p>
						<p class="text-red-700 text-sm">{searchError}</p>
					</div>
				{/if}

				{#if searchResults}
					<div class="space-y-3">
						<h3 class="font-medium">Results:</h3>
						<div class="bg-muted p-3 rounded-md max-h-64 overflow-y-auto">
							<pre class="text-xs">{JSON.stringify(searchResults, null, 2)}</pre>
						</div>
						
						{#if searchResults.results}
							<div class="text-sm text-muted-foreground">
								Found results from: {Object.keys(searchResults.results).join(', ')}
							</div>
						{/if}
					</div>
				{/if}
			</CardContent>
		</Card>

		<!-- Resolve Test -->
		<Card>
			<CardHeader>
				<CardTitle>Resolve Song URL</CardTitle>
			</CardHeader>
			<CardContent class="space-y-4">
				<div class="space-y-2">
					<label for="resolve-url" class="text-sm font-medium">Platform URL</label>
					<Input
						id="resolve-url"
						bind:value={resolveUrl}
						placeholder="https://open.spotify.com/track/..."
					/>
				</div>
				
				<Button 
					onclick={testResolve} 
					disabled={resolveLoading || !resolveUrl.trim()}
					class="w-full"
				>
					{resolveLoading ? 'Resolving...' : 'Test Resolve'}
				</Button>

				{#if resolveError}
					<div class="p-3 bg-red-50 border border-red-200 rounded-md">
						<p class="text-red-800 text-sm font-medium">Error:</p>
						<p class="text-red-700 text-sm">{resolveError}</p>
					</div>
				{/if}

				{#if resolveResults}
					<div class="space-y-3">
						<h3 class="font-medium">Results:</h3>
						<div class="bg-muted p-3 rounded-md max-h-64 overflow-y-auto">
							<pre class="text-xs">{JSON.stringify(resolveResults, null, 2)}</pre>
						</div>
						
						{#if resolveResults.song}
							<div class="text-sm text-muted-foreground">
								Song: {resolveResults.song.title} by {resolveResults.song.artists?.join(', ')}
							</div>
						{/if}
					</div>
				{/if}
			</CardContent>
		</Card>
	</div>

	<!-- API Status -->
	<Card class="mt-6">
		<CardHeader>
			<CardTitle>API Configuration</CardTitle>
		</CardHeader>
		<CardContent>
			<div class="space-y-2 text-sm">
				<div><strong>Base URL:</strong> http://localhost:8080/api/v1</div>
				<div><strong>Available Endpoints:</strong></div>
				<ul class="list-disc list-inside ml-4 space-y-1">
					<li>POST /songs/search - Search for songs across platforms</li>
					<li>POST /songs/resolve - Convert platform URL to universal song</li>
					<li>GET /s/:id - Universal song links</li>
				</ul>
			</div>
		</CardContent>
	</Card>
</div>