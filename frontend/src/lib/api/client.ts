import type { paths } from '$lib/types/api';

// Type-safe API client using the generated OpenAPI types
type ApiPaths = paths;

// Extract request/response types for each endpoint
export type ResolveSongRequest = ApiPaths['/songs/resolve']['post']['requestBody']['content']['application/json'];
export type ResolveSongResponse = ApiPaths['/songs/resolve']['post']['responses']['200']['content']['application/json'];

export type SearchSongsRequest = ApiPaths['/songs/search']['post']['requestBody']['content']['application/json'];
export type SearchSongsResponse = ApiPaths['/songs/search']['post']['responses']['200']['content']['application/json'];

export type Song = ApiPaths['/songs/{songId}']['get']['responses']['200']['content']['application/json'];
export type PlatformLink = Song['platforms'][0];
export type SearchResult = SearchSongsResponse['results'][string][0];

// API Configuration
const API_BASE_URL = 'http://localhost:8080/api/v1';

// Generic API client
class ApiClient {
	private baseUrl: string;

	constructor(baseUrl: string = API_BASE_URL) {
		this.baseUrl = baseUrl;
	}

	private async request<T>(
		endpoint: string,
		options: RequestInit = {}
	): Promise<T> {
		const url = `${this.baseUrl}${endpoint}`;
		
		const response = await fetch(url, {
			headers: {
				'Content-Type': 'application/json',
				...options.headers,
			},
			...options,
		});

		if (!response.ok) {
			const error = await response.json().catch(() => ({ 
				error: 'Request failed',
				details: response.statusText 
			}));
			throw new ApiError(response.status, error.error || 'Request failed', error.details);
		}

		return response.json();
	}

	// Song resolution
	async resolveSong(request: ResolveSongRequest): Promise<ResolveSongResponse> {
		return this.request<ResolveSongResponse>('/songs/resolve', {
			method: 'POST',
			body: JSON.stringify(request),
		});
	}

	// Song search
	async searchSongs(request: SearchSongsRequest): Promise<SearchSongsResponse> {
		return this.request<SearchSongsResponse>('/songs/search', {
			method: 'POST',
			body: JSON.stringify(request),
		});
	}

	// Get song by ID/ISRC
	async getSong(songId: string): Promise<Song> {
		return this.request<Song>(`/songs/${encodeURIComponent(songId)}`);
	}

	// Health check
	async healthCheck(): Promise<{ status: string; timestamp: string }> {
		return this.request<{ status: string; timestamp: string }>('/health');
	}
}

// Custom error class for API errors
export class ApiError extends Error {
	constructor(
		public status: number,
		message: string,
		public details?: string
	) {
		super(message);
		this.name = 'ApiError';
	}
}

// Export singleton instance
export const apiClient = new ApiClient();

// Export types for use in components
export type { Song, PlatformLink, SearchResult };