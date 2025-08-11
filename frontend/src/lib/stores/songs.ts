/**
 * Svelte stores for managing song-related state
 */

import { writable, derived, type Readable } from 'svelte/store';
import type { Song, SearchResult, SearchQuery } from '$lib/api/client';

// Search state
export const searchQuery = writable<string>('');
export const searchResults = writable<SearchResult | null>(null);
export const isSearching = writable<boolean>(false);
export const searchError = writable<string | null>(null);

// Current song state
export const currentSong = writable<Song | null>(null);
export const isLoadingSong = writable<boolean>(false);
export const songError = writable<string | null>(null);

// Search history (stored in localStorage)
export const searchHistory = writable<string[]>([]);

// Popular and recent songs
export const popularSongs = writable<Song[]>([]);
export const recentSongs = writable<Song[]>([]);

// URL resolution state
export const isResolvingUrl = writable<boolean>(false);
export const resolveError = writable<string | null>(null);

// Derived stores
export const hasSearchResults: Readable<boolean> = derived(
	searchResults,
	($searchResults) => $searchResults !== null && $searchResults.songs.length > 0
);

export const searchResultsCount: Readable<number> = derived(
	searchResults,
	($searchResults) => $searchResults?.total || 0
);

export const hasMoreResults: Readable<boolean> = derived(
	searchResults,
	($searchResults) => $searchResults?.hasMore || false
);

// Actions for managing search history
export const searchHistoryActions = {
	add: (query: string) => {
		if (!query.trim()) return;
		
		searchHistory.update(history => {
			const newHistory = [query, ...history.filter(item => item !== query)];
			return newHistory.slice(0, 10); // Keep only last 10 searches
		});
	},
	
	remove: (query: string) => {
		searchHistory.update(history => history.filter(item => item !== query));
	},
	
	clear: () => {
		searchHistory.set([]);
	}
};

// Load search history from localStorage on initialization
if (typeof window !== 'undefined') {
	const savedHistory = localStorage.getItem('songshare-search-history');
	if (savedHistory) {
		try {
			searchHistory.set(JSON.parse(savedHistory));
		} catch (error) {
			console.warn('Failed to load search history from localStorage:', error);
		}
	}

	// Save search history to localStorage whenever it changes
	searchHistory.subscribe(history => {
		try {
			localStorage.setItem('songshare-search-history', JSON.stringify(history));
		} catch (error) {
			console.warn('Failed to save search history to localStorage:', error);
		}
	});
}