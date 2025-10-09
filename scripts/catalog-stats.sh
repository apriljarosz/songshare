#!/bin/bash

# Catalog statistics script

echo "🎵 SongShare Catalog Statistics"
echo "================================"
echo ""

# Total songs
total=$(docker exec songshare-mongodb mongosh --quiet --eval "db = db.getSiblingDB('songshare'); db.songs.countDocuments()")
echo "Total songs: $total"

# Songs with both platforms
both=$(docker exec songshare-mongodb mongosh --quiet --eval "db = db.getSiblingDB('songshare'); db.songs.countDocuments({'platforms.1': {\$exists: true}})")
echo "Songs with both platforms: $both"

# Spotify only
spotify_only=$(docker exec songshare-mongodb mongosh --quiet --eval "db = db.getSiblingDB('songshare'); db.songs.countDocuments({platforms: {\$size: 1}, 'platforms.0.platform': 'spotify'})")
echo "Spotify only: $spotify_only"

# Apple Music only
apple_only=$(docker exec songshare-mongodb mongosh --quiet --eval "db = db.getSiblingDB('songshare'); db.songs.countDocuments({platforms: {\$size: 1}, 'platforms.0.platform': 'apple_music'})")
echo "Apple Music only: $apple_only"

echo ""
echo "Recent additions:"
docker exec songshare-mongodb mongosh --quiet --eval "db = db.getSiblingDB('songshare'); db.songs.find({}, {title: 1, artists: 1, 'platforms.platform': 1, created_at: 1}).sort({created_at: -1}).limit(5).forEach(s => print(s.title + ' - ' + s.artists[0] + ' [' + s.platforms.map(p => p.platform).join(', ') + ']'))"
