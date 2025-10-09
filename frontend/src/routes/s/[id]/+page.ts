import apiClient from '$lib/api/client';
import type { PageLoad } from './$types';

export const load: PageLoad = async ({ params }) => {
	try {
		const song = await apiClient.getSong(params.id);
		return { song };
	} catch (error) {
		return {
			error: error instanceof Error ? error.message : 'Failed to load song',
		};
	}
};
