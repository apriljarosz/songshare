#!/bin/bash

# Database Statistics Script for SongShare
# This script provides comprehensive database size tracking and monitoring

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

echo -e "${BLUE}🗄️  SongShare Database Statistics${NC}"
echo "======================================="
echo ""

# Check if MongoDB container is running
if ! docker ps | grep -q "songshare_mongodb"; then
    echo -e "${RED}❌ MongoDB container not running${NC}"
    echo "Run: docker-compose up -d"
    exit 1
fi

echo -e "${GREEN}✅ MongoDB container is running${NC}"
echo ""

# Get database statistics
echo -e "${YELLOW}📊 Database Overview${NC}"
docker exec songshare_mongodb mongosh --quiet \
    -u admin -p password --authenticationDatabase admin \
    songshare --eval "
// Get database stats
try {
    var dbStats = db.stats()
    var totalSizeMB = Math.round((dbStats.dataSize || 0) / 1024 / 1024 * 100) / 100
    var storageSizeMB = Math.round((dbStats.storageSize || 0) / 1024 / 1024 * 100) / 100  
    var indexSizeMB = Math.round((dbStats.indexSize || 0) / 1024 / 1024 * 100) / 100

    print('Database Name: ' + db.getName())
    print('Total Data Size: ' + totalSizeMB + ' MB')
    print('Storage Size: ' + storageSizeMB + ' MB') 
    print('Index Size: ' + indexSizeMB + ' MB')
    print('Total Collections: ' + db.getCollectionNames().length)
    print('Total Documents: ' + (dbStats.objects || 0))
    print('')

    // Collection details
    print('📋 Collections:')
    var collections = db.getCollectionNames()
    if (collections.length === 0) {
        print('  (No collections found - database is empty)')
    } else {
        collections.forEach(function(collectionName) {
            try {
                var stats = db.getCollection(collectionName).stats()
                var sizeMB = Math.round((stats.size || 0) / 1024 / 1024 * 100) / 100
                var storageMB = Math.round((stats.storageSize || 0) / 1024 / 1024 * 100) / 100
                print('  ' + collectionName + ': ' + (stats.count || 0) + ' docs, ' + sizeMB + ' MB data, ' + storageMB + ' MB storage')
            } catch(e) {
                print('  ' + collectionName + ': Error getting stats - ' + e.message)
            }
        })
    }
} catch(error) {
    print('Error getting database stats: ' + error.message)
}
"

echo ""

# Get most recent songs
echo -e "${YELLOW}🎵 Recent Activity${NC}"
docker exec songshare_mongodb mongosh --quiet \
    -u admin -p password --authenticationDatabase admin \
    songshare --eval "
try {
    if (db.songs && db.songs.countDocuments() > 0) {
        print('Recent songs:')
        db.songs.find({}, {title: 1, artist: 1, created_at: 1})
               .sort({created_at: -1})
               .limit(5)
               .forEach(function(song) {
                   var date = song.created_at ? song.created_at.toISOString().split('T')[0] : 'N/A'
                   print('  ' + (song.title || 'Untitled') + ' by ' + (song.artist || 'Unknown') + ' (' + date + ')')
               })
    } else {
        print('No songs in database yet - start using the API to add songs!')
    }
} catch(error) {
    print('No songs collection found - database is empty')
}
"

echo ""

# Cache statistics
echo -e "${YELLOW}💾 Cache Statistics (Valkey)${NC}"
if docker ps | grep -q "songshare_valkey"; then
    docker exec songshare_valkey valkey-cli --pass valkey123 INFO memory | grep -E "used_memory_human|used_memory_peak_human|maxmemory_human" || echo "Cache statistics unavailable"
    echo -n "Cache keys: "
    docker exec songshare_valkey valkey-cli --pass valkey123 DBSIZE
else
    echo -e "${RED}❌ Valkey container not running${NC}"
fi

echo ""

# Growth tracking (if log file exists)
STATS_LOG="/tmp/songshare-stats.log"
if [ -f "$STATS_LOG" ]; then
    echo -e "${YELLOW}📈 Growth Tracking${NC}"
    echo "Recent size changes:"
    tail -5 "$STATS_LOG"
else
    echo -e "${BLUE}💡 Tip: Run this script regularly to track growth over time${NC}"
fi

# Save current stats to log for growth tracking
TIMESTAMP=$(date '+%Y-%m-%d %H:%M:%S')
DB_SIZE=$(docker exec songshare_mongodb mongosh --quiet --host localhost --port 27017 -u admin -p password --authenticationDatabase admin --eval "use songshare; Math.round(db.stats().dataSize / 1024 / 1024 * 100) / 100" 2>/dev/null || echo "0")
DOC_COUNT=$(docker exec songshare_mongodb mongosh --quiet --host localhost --port 27017 -u admin -p password --authenticationDatabase admin --eval "use songshare; db.stats().objects" 2>/dev/null || echo "0")
echo "$TIMESTAMP | Size: ${DB_SIZE} MB | Docs: $DOC_COUNT" >> "$STATS_LOG"

echo ""
echo -e "${GREEN}✅ Statistics saved to $STATS_LOG${NC}"
echo -e "${BLUE}💡 Run 'docker exec songshare_mongodb mongosh --host localhost -u admin -p password --authenticationDatabase admin' to access MongoDB shell${NC}"