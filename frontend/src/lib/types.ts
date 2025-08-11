export interface Song {
	id: string;
	schema_version: number;
	isrc: string;
	title: string;
	artist: string;
	album?: string;
	platform_links: PlatformLink[];
	metadata: SongMetadata;
	created_at: string;
	updated_at: string;
}

export interface PlatformLink {
	platform: string;
	external_id: string;
	url: string;
	available: boolean;
	confidence: number;
	last_verified: string;
}

export interface SongMetadata {
	genre?: string[];
	duration_ms?: number;
	release_date?: string;
	language?: string;
	popularity?: number;
	explicit?: boolean;
	image_url?: string;
}

export type Platform = 'spotify' | 'apple_music' | 'tidal' | 'youtube_music' | 'deezer';

export const PLATFORM_NAMES: Record<Platform, string> = {
	spotify: 'Spotify',
	apple_music: 'Apple Music',
	tidal: 'Tidal',
	youtube_music: 'YouTube Music',
	deezer: 'Deezer'
};

export const PLATFORM_COLORS: Record<Platform, string> = {
	spotify: 'bg-green-500',
	apple_music: 'bg-gray-900',
	tidal: 'bg-blue-500',
	youtube_music: 'bg-red-500',
	deezer: 'bg-purple-500'
};