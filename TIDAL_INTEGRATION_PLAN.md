# Tidal API Integration Plan

## Current Status: ✅ Authentication Working, ❌ Wrong Headers/Endpoints

### Key Discoveries
1. **OAuth2 Client Credentials flow works correctly**
2. **Wrong Content-Type headers** - Need `application/vnd.api+json` not `application/vnd.tidal.v1+json`
3. **JSON:API response format** - Uses `data.attributes` structure, not flat objects
4. **Include parameter** - Can request related data with `?include=albums,artists`

---

## API Endpoint Mapping

### ✅ 1. GetTrackByID - WORKING
**Current Implementation:** 
```
GET https://openapi.tidal.com/v2/tracks/{trackID}?countryCode=US
Headers: application/vnd.tidal.v1+json (WRONG)
```

**Correct Implementation:**
```
GET /tracks/{id}?countryCode=US&include=albums,artists
Headers: 
  - Accept: application/vnd.api+json
  - Authorization: Bearer {token}
```

**Response Format:**
```json
{
  "data": {
    "id": "75413016",
    "type": "tracks", 
    "attributes": {
      "title": "4:44",
      "isrc": "QMJMT1701237",
      "duration": "PT4M44S",
      "explicit": true,
      "popularity": 0.777,
      "externalLinks": [{"href": "https://tidal.com/browse/track/75413016"}]
    },
    "relationships": { ... }
  },
  "included": [
    { "type": "artists", "attributes": {"name": "JAY Z"} },
    { "type": "albums", "attributes": {"title": "4:44"} }
  ]
}
```

**Action Items:**
- [ ] Update headers to `application/vnd.api+json`
- [ ] Add `include=albums,artists` parameter
- [ ] Update response parsing for JSON:API format
- [ ] Create new struct for JSON:API response format

---

### ✅ 2. GetTrackByISRC - WORKING
**Current Implementation:**
```
SearchTrack -> GetTrackByID (inefficient)
```

**Correct Implementation:**
```
GET /tracks?countryCode=US&filter[isrc]={isrc}&include=albums,artists
URL Encoded: filter%5Bisrc%5D={isrc}
```

**Response Format:**
```json
{
  "data": [
    {
      "id": "75413016",
      "type": "tracks",
      "attributes": { ... }
    }
  ]
}
```

**Action Items:**
- [ ] Replace SearchTrack call with direct filter query
- [ ] Add proper URL encoding for filter parameters
- [ ] Handle array response format

---

### ✅ 3. SearchTrack - UNDERSTANDING SEARCH FLOW
**Current Implementation:**
```
GET https://openapi.tidal.com/v2/searchresults?query={term}&type=tracks&limit={N}&countryCode=US
```

**Status:** ❌ Wrong endpoint structure - search uses query as ID!

**🔍 KEY DISCOVERY: The search query IS the ID!**
- `GET /searchResults/{id}` where `{id}` is the search term (e.g., "moon", "bohemian rhapsody")
- Example: `GET /searchResults/moon?countryCode=US`

**Search Flow:**
```
1. GET /searchResults/{searchQuery}?countryCode=US&include=tracks
   - Returns search metadata + track IDs in relationships
   - Can include: albums, artists, playlists, topHits, tracks, videos
   
2. Optional: GET /searchResults/{searchQuery}/relationships/tracks  
   - Get full track details from relationships
```

**Parameters:**
- `countryCode` (required): ISO country code (US)  
- `explicitFilter`: include/exclude explicit content
- `include`: Return related data (tracks, albums, artists, etc.)

**Response Structure:**
```json
{
  "data": {
    "id": "moon",
    "type": "searchResults", 
    "attributes": {
      "didYouMean": "beatles",
      "trackingId": "5896e37d-e847-4ca6-9629-ef8001719f7f"
    },
    "relationships": {
      "tracks": {
        "data": [{"id": "12345", "type": "tracks"}],
        "links": {"self": "/searchResults/moon/relationships/tracks"}
      }
    }
  },
  "included": [
    // Full track/artist/album details if include parameter used
  ]
}
```

**Two Search Approaches:**

**Option A: Single Call (Recommended)**
```
GET /searchResults/{query}?countryCode=US&include=tracks
- Returns track IDs in relationships AND full track details in included array
- More efficient, single API call
- Limited to ~20 tracks per response with pagination
```

**Option B: Two Calls (More Control)**
```
1. GET /searchResults/{query}?countryCode=US  
   - Returns track IDs in relationships.tracks.data[]
   
2. GET /searchResults/{query}/relationships/tracks?countryCode=US&include=tracks
   - Returns only track IDs with pagination support
   - Need separate calls to get full track details
```

**Current Test Results:**
✅ **Working:** `GET /searchResults/bohemian%20rhapsody?countryCode=US&include=tracks`
- Returns 20 track IDs + full track details in included array
- Has pagination with `links.next`
- Track details have: title, ISRC, duration, popularity, etc.
- Missing artist/album names (only IDs in relationships)

**Action Items:**
- [x] Confirmed search endpoint works with include=tracks  
- [ ] Test getting artist/album details via track relationships
- [ ] Implement SearchTrack using Option A (single call)
- [ ] Handle pagination for more results

---

### 4. Health Check
**Current Implementation:**
```
GET https://openapi.tidal.com/v2/tracks/77646168?countryCode=US (hardcoded track)
```

**Planned Update:**
```
GET /tracks/75413016?countryCode=US (use working track ID)
Headers: application/vnd.api+json
```

**Action Items:**
- [ ] Update to use working track ID (75413016)
- [ ] Fix headers
- [ ] Update health check parsing

---

## JSON:API Response Structures Needed

### TidalTrackResponse (JSON:API format)
```go
type TidalTrackResponse struct {
    Data     TidalTrackData   `json:"data"`
    Included []TidalIncluded  `json:"included,omitempty"`
    Links    TidalLinks       `json:"links,omitempty"`
}

type TidalTracksResponse struct {
    Data     []TidalTrackData `json:"data"`
    Included []TidalIncluded  `json:"included,omitempty"`
    Links    TidalLinks       `json:"links,omitempty"`
}

type TidalTrackData struct {
    ID            string                 `json:"id"`
    Type          string                 `json:"type"`
    Attributes    TidalTrackAttributes   `json:"attributes"`
    Relationships TidalRelationships     `json:"relationships"`
}

type TidalTrackAttributes struct {
    Title         string                 `json:"title"`
    ISRC          string                 `json:"isrc"`
    Duration      string                 `json:"duration"` // ISO 8601 format "PT4M44S"
    Copyright     string                 `json:"copyright"`
    Explicit      bool                   `json:"explicit"`
    Popularity    float64                `json:"popularity"`
    Availability  []string               `json:"availability"`
    ExternalLinks []TidalExternalLink    `json:"externalLinks"`
}

type TidalIncluded struct {
    ID         string                 `json:"id"`
    Type       string                 `json:"type"` // "artists" or "albums"
    Attributes map[string]interface{} `json:"attributes"`
}
```

---

## Implementation Priority

1. **HIGH: Fix GetTrackByID** - Change headers and response parsing
2. **HIGH: Fix GetTrackByISRC** - Use filter endpoint instead of search
3. **MEDIUM: Get Search specification** - Need search endpoint details
4. **LOW: Update Health check** - Use working track ID

---

## Testing Status

### ✅ Working API Calls
```bash
# GetTrackByID
curl -H "accept: application/vnd.api+json" \
     -H "Authorization: Bearer {token}" \
     "https://openapi.tidal.com/v2/tracks/75413016?countryCode=US&include=albums,artists"

# GetTrackByISRC  
curl -H "accept: application/vnd.api+json" \
     -H "Authorization: Bearer {token}" \
     "https://openapi.tidal.com/v2/tracks?countryCode=US&filter%5Bisrc%5D=QMJMT1701237"
```

### ❌ Not Working
- Search endpoint (need specification)
- Current service implementation (wrong headers)

---

## Next Steps

1. Get search endpoint specification
2. Update TidalService struct definitions
3. Fix headers in all API calls
4. Update response parsing logic
5. Test with real API calls
6. Update error handling for new response format