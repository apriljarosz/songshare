# SongShare Search Architecture Improvements

## Current Issues

1. **Platform Bias**: Auto-indexing creates permanent bias toward first platform that responds
2. **Stale Results**: Over-aggressive caching (30min search + permanent local storage)
3. **Limited Platform Coverage**: Sequential fallback instead of parallel search
4. **Poor Result Freshness**: No indication of result age or source

## Proposed Improvements

### 1. Parallel Platform Search Strategy

Instead of: Local DB → Platform APIs (sequential)
Implement: **Parallel Multi-Source Search**

```
Search Query
├── Local DB (immediate)
├── Spotify API (parallel)
└── Apple Music API (parallel)
```

**Benefits**:
- Faster response times
- Better platform coverage
- More balanced results

### 2. Smart Caching Strategy

**Current**: Everything cached for 30 minutes + permanent local storage
**Proposed**: Tiered caching with freshness indicators

- **Hot cache** (1-5 minutes): Popular searches
- **Warm cache** (15-30 minutes): Regular searches  
- **Cold cache** (1-6 hours): Rare searches
- **Platform cache** with explicit TTL and refresh logic

### 3. Result Ranking & Deduplication

**Enhanced grouping logic**:
- ISRC matching (primary)
- Title + Artist similarity (secondary)
- Duration matching (tertiary)
- User preference weighting

### 4. Platform Balance Features

- **Round-robin platform prioritization**
- **Circuit breaker** for failing services
- **Result source indicators** (fresh vs cached)
- **Platform availability badges** for all discovered sources

### 5. Search Result Presentation

- Show **all available platforms** with clickable badges
- **Freshness indicators** (cached 5min ago, live from Spotify, etc.)
- **Universal link creation** on-demand, not auto-indexing
- **Source attribution** (found via Spotify, cached locally, etc.)

## Implementation Plan

### Phase 1: Parallel Search (Quick Win)
1. Modify `SearchResults` to search platforms in parallel
2. Aggregate and rank results before grouping
3. Add result source tracking

### Phase 2: Smart Caching
1. Implement tiered cache TTLs
2. Add cache freshness metadata
3. Background refresh for popular searches

### Phase 3: Enhanced UX
1. Add platform balance indicators
2. Implement user platform preferences
3. Add search result monitoring/analytics

## Quick Fix for Current Issue

**Immediate solution** for the "only Apple Music + SongShare" problem:

1. **Disable auto-indexing** for search results (only index when user explicitly creates universal link)
2. **Reduce search cache TTL** from 30 minutes to 2-5 minutes
3. **Implement parallel platform search** to get fresh results from both platforms
4. **Show all platform badges** found during search, not just stored ones

This will give you more balanced, fresher results while maintaining performance.