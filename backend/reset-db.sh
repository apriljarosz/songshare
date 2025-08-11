#!/bin/bash

# Quick script to reset database and cache for testing (databases only)
echo "🔄 Stopping database services and clearing all data..."
docker-compose stop mongodb valkey
docker-compose rm -f mongodb valkey

echo "🗑️ Removing database volumes..."
docker volume rm songshare_mongo-data songshare_mongo-config songshare_valkey-data 2>/dev/null || true

echo "🔨 Starting fresh database services..."
docker-compose up -d mongodb valkey

echo "⏳ Waiting for database services to be healthy..."
echo "Waiting for MongoDB..."
until docker-compose exec mongodb mongosh --eval "db.adminCommand('ping')" >/dev/null 2>&1; do
    echo -n "."
    sleep 2
done
echo " MongoDB ready!"

echo "Waiting for Valkey..."
until docker-compose exec valkey valkey-cli ping >/dev/null 2>&1; do
    echo -n "."
    sleep 2
done
echo " Valkey ready!"

echo "✅ Database and cache reset complete!"
echo "💡 Note: Go application (if running with 'air') will automatically reconnect to the fresh databases"