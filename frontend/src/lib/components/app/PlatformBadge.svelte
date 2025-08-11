<script lang="ts">
	import { Badge } from '$lib/components/ui/badge';
	import { PLATFORM_NAMES, PLATFORM_COLORS, type PlatformLink } from '$lib/types/generated';

	interface Props {
		link: PlatformLink;
		class?: string;
	}

	let { link, class: className = '' }: Props = $props();

	const platformName = PLATFORM_NAMES[link.platform] || link.platform;
	const baseColor = PLATFORM_COLORS[link.platform] || 'bg-gray-500';
	
	// Adjust opacity based on availability and confidence
	let badgeClass = $state(baseColor);
	$effect(() => {
		badgeClass = baseColor;
		if (!link.available) {
			badgeClass += ' opacity-50';
		} else if (link.confidence < 0.8) {
			badgeClass += ' opacity-75';
		}
	});
</script>

<Badge 
	variant="secondary" 
	class="{badgeClass} text-white hover:opacity-80 transition-opacity {className}"
	title={`Confidence: ${Math.round(link.confidence * 100)}%`}
>
	{platformName}
	{#if !link.available}
		<span class="ml-1 text-xs opacity-75">(unavailable)</span>
	{/if}
</Badge>