// Auto-generated TypeScript types from Go structs
// Generated from: songshare/internal/models

export interface Song {
	id: string;
	title: string;
	artist: string;
	album?: string;
	duration?: number; // duration in seconds
	isrc?: string;
	albumArt?: string;
	platforms: PlatformLink[];
	createdAt: Date;
	updatedAt: Date;
}

export interface PlatformLink {
	platform: Platform;
	url: string;
	available: boolean;
	confidence: number;
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

// Platform types and constants
export type Platform = 'spotify' | 'apple_music' | 'tidal' | 'youtube_music' | 'deezer';

export const PLATFORMS = {
	SPOTIFY: 'spotify' as Platform,
	APPLE_MUSIC: 'apple_music' as Platform,
	TIDAL: 'tidal' as Platform,
	YOUTUBE_MUSIC: 'youtube_music' as Platform,
	DEEZER: 'deezer' as Platform
} as const;

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