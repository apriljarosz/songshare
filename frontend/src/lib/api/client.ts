import type { Song, SearchRequest, ResolveRequest, Platform } from '$lib/types/generated';

const API_BASE_URL = import.meta.env.VITE_API_URL || 'http://localhost:8080';

interface SearchResponse {
	songs: Song[];
}

interface ResolveResponse {
	song: Song;
}

interface PlatformsResponse {
	platforms: Platform[];
}

class APIClient {
	private baseURL: string;

	constructor(baseURL: string = API_BASE_URL) {
		this.baseURL = baseURL;
	}

	private async request<T>(endpoint: string, options?: RequestInit): Promise<T> {
		const url = `${this.baseURL}${endpoint}`;

		try {
			const response = await fetch(url, {
				...options,
				headers: {
					'Content-Type': 'application/json',
					...options?.headers,
				},
			});

			if (!response.ok) {
				const errorData = await response.json().catch(() => ({ error: response.statusText }));
				throw new Error(errorData.error || `HTTP ${response.status}: ${response.statusText}`);
			}

			return await response.json();
		} catch (error) {
			if (error instanceof Error) {
				throw error;
			}
			throw new Error('An unknown error occurred');
		}
	}

	async search(query: string, offset: number = 0, limit: number = 10): Promise<Song[]> {
		const body: SearchRequest = { query, offset, limit };
		const response = await this.request<SearchResponse>('/api/search', {
			method: 'POST',
			body: JSON.stringify(body),
		});
		return response.songs;
	}

	async resolve(url: string): Promise<Song> {
		const body: ResolveRequest = { url };
		const response = await this.request<ResolveResponse>('/api/resolve', {
			method: 'POST',
			body: JSON.stringify(body),
		});
		return response.song;
	}

	async getSong(id: string): Promise<Song> {
		return await this.request<Song>(`/api/songs/${id}`);
	}

	async getPlatforms(): Promise<Platform[]> {
		const response = await this.request<PlatformsResponse>('/api/platforms');
		return response.platforms;
	}

	async healthCheck(): Promise<{ status: string }> {
		return await this.request<{ status: string }>('/health');
	}
}

export const apiClient = new APIClient();
export default apiClient;
