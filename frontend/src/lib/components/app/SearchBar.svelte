<script lang="ts">
	import { Input } from '$lib/components/ui/input';
	import { Button } from '$lib/components/ui/button';
	import { Search, X, Loader2 } from 'lucide-svelte';

	interface Props {
		value?: string;
		placeholder?: string;
		loading?: boolean;
		class?: string;
		onSearch?: (query: string) => void;
		onClear?: () => void;
	}

	let { 
		value = '', 
		placeholder = 'Search for songs, artists, or albums...', 
		loading = false,
		class: className = '',
		onSearch,
		onClear
	}: Props = $props();

	let inputValue = $state(value);
	let inputElement: HTMLInputElement;

	// Update internal state when prop changes
	$effect(() => {
		inputValue = value;
	});

	function handleSubmit(event: Event) {
		event.preventDefault();
		if (inputValue.trim() && onSearch) {
			onSearch(inputValue.trim());
		}
	}

	function handleClear() {
		inputValue = '';
		if (onClear) {
			onClear();
		}
		inputElement?.focus();
	}

	function handleKeydown(event: KeyboardEvent) {
		if (event.key === 'Escape') {
			handleClear();
		}
	}
</script>

<form onsubmit={handleSubmit} class="relative {className}">
	<div class="relative">
		<Search class="absolute left-3 top-1/2 transform -translate-y-1/2 text-muted-foreground w-4 h-4" />
		
		<Input
			bind:this={inputElement}
			bind:value={inputValue}
			onkeydown={handleKeydown}
			{placeholder}
			disabled={loading}
			class="pl-10 pr-20 h-12 text-base"
		/>
		
		<div class="absolute right-2 top-1/2 transform -translate-y-1/2 flex items-center gap-1">
			{#if loading}
				<Loader2 class="w-4 h-4 animate-spin text-muted-foreground" />
			{:else if inputValue}
				<Button
					type="button"
					variant="ghost"
					size="sm"
					onclick={handleClear}
					class="h-8 w-8 p-0 hover:bg-muted"
				>
					<X class="w-4 h-4" />
					<span class="sr-only">Clear search</span>
				</Button>
			{/if}
			
			<Button
				type="submit"
				size="sm"
				disabled={loading || !inputValue.trim()}
				class="h-8 px-3"
			>
				{#if loading}
					<Loader2 class="w-4 h-4 animate-spin" />
				{:else}
					<Search class="w-4 h-4" />
				{/if}
				<span class="sr-only">Search</span>
			</Button>
		</div>
	</div>
</form>