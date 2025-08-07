# SongShare Database Monitoring

This directory contains tools and utilities for monitoring your SongShare database size and growth.

## 🗄️ Current Database Size

Your database is currently **empty** (0 MB) - this is expected for a new installation.

## 📊 Monitoring Tools

### 1. Database Statistics Script (`../scripts/db-stats.sh`)
Quick command-line tool to check database size and growth:

```bash
./scripts/db-stats.sh
```

**What it shows:**
- Total database size (MB)
- Storage size and index size
- Document counts per collection
- Recent activity
- Cache statistics (Valkey)
- Growth tracking over time

### 2. Admin Web Interface (`/admin/db-stats`)
Real-time web dashboard (when implemented in your routes):

- Interactive charts and graphs
- Real-time statistics
- Growth projections
- Auto-refresh every 30 seconds

### 3. MongoDB Direct Access
Connect directly to your database:

```bash
docker exec -it songshare_mongodb mongosh --host localhost -u admin -p password --authenticationDatabase admin
```

Then use MongoDB commands:
```javascript
use songshare
db.stats()                    // Database statistics
db.songs.countDocuments()     // Count songs
db.songs.find().limit(5)      // Recent songs
```

## 📈 Tracking Database Growth

### Expected Growth Patterns

**Light Usage (1-10 songs/day):**
- ~1-5 MB per 1000 songs
- Mostly metadata with some cached images
- Index overhead: ~10-20% of data size

**Moderate Usage (10-100 songs/day):**
- ~5-20 MB per 1000 songs  
- More platform links and metadata
- Cache hit rates improve efficiency

**Heavy Usage (100+ songs/day):**
- ~20-50 MB per 1000 songs
- Comprehensive album art caching
- Search index optimization becomes important

### Size Estimates by Song Count

| Songs | Data Size | Storage Size | Notes |
|-------|-----------|--------------|-------|
| 100   | 0.5 MB    | 1 MB        | Basic metadata only |
| 1,000 | 5 MB      | 10 MB       | Some album art cached |
| 10,000| 50 MB     | 80 MB       | Full platform integration |
| 100,000| 500 MB   | 800 MB      | Comprehensive search index |

## 🚨 Monitoring Alerts

### When to Pay Attention

1. **Size > 100 MB**: Time to implement data archiving
2. **Growth > 10 MB/day**: Check for runaway indexing
3. **Index size > 50% of data**: Optimize queries
4. **Cache hit rate < 70%**: Tune cache settings

### Optimization Strategies

**Small Database (< 10 MB):**
- No optimization needed
- Focus on functionality

**Medium Database (10-100 MB):**
- Monitor query performance
- Consider index optimization
- Implement cache warming

**Large Database (> 100 MB):**
- Data lifecycle policies
- Archive old songs
- Shard by platform or date
- Connection pooling

## 🔧 Database Maintenance

### Regular Tasks

**Daily:**
- Check growth rate with `./scripts/db-stats.sh`
- Monitor application logs for slow queries

**Weekly:**
- Review index usage: `db.songs.getIndexes()`
- Check for duplicate songs
- Validate cache hit rates

**Monthly:**
- Full database backup
- Index defragmentation
- Performance benchmarking

### Automated Monitoring Setup

Add to your `docker-compose.yml`:
```yaml
services:
  db-monitor:
    image: mongo:8.0
    command: sh -c "while true; do mongosh --host mongodb --eval 'use songshare; print(new Date(), \"Size:\", Math.round(db.stats().dataSize/1024/1024*100)/100, \"MB\")' >> /logs/db-growth.log; sleep 3600; done"
    volumes:
      - ./logs:/logs
    depends_on:
      - mongodb
```

## 📱 API Integration

Monitor via your application:
```bash
# Current stats
curl http://localhost:8080/api/v1/admin/db-stats

# Web dashboard  
open http://localhost:8080/admin/db-stats
```

## 🎯 Growth Projections

Based on typical usage patterns:

- **Personal use**: < 1 MB/month
- **Small team**: 1-10 MB/month  
- **Production app**: 10-100 MB/month
- **High-traffic**: 100+ MB/month

Your database will likely remain under 10 MB for the first few months of development and testing.