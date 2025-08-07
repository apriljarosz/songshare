#!/bin/bash

# Quick script to reset database and cache for testing
echo "🔄 Stopping services and clearing all data..."
docker-compose down -v

echo "🔨 Rebuilding services with fresh containers..."
docker-compose up --build -d

echo "⏳ Waiting for services to be healthy..."
sleep 15

echo "✅ Database and cache reset complete!"
echo "🌐 Search page: http://localhost:8080/search"
echo ""
echo "Testing Apple Music authentication..."
sleep 3

# Test if Apple Music is working
if curl -s "http://localhost:8080/api/v1/search/results?q=test" | grep -q "music.apple.com"; then
    echo "✅ Apple Music: Working"
else 
    echo "❌ Apple Music: Authentication issue detected"
fi