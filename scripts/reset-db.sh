#!/bin/bash

# Reset MongoDB database for development
echo "🗑️  Resetting MongoDB database..."

# Check if MongoDB container is running
if ! docker ps | grep -q songshare-mongodb; then
    echo "❌ MongoDB container is not running. Starting it..."
    docker-compose up mongodb -d
    sleep 3
fi

# Drop the database
docker-compose exec -T mongodb mongosh songshare --eval "db.dropDatabase()" --quiet

if [ $? -eq 0 ]; then
    echo "✅ Database reset successfully!"
    echo "📊 The database will be rebuilt as you search for songs."
else
    echo "❌ Failed to reset database"
    exit 1
fi
