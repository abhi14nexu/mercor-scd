#!/bin/bash

echo "🚀 Starting SCD Dashboard Environment..."
echo ""

# Check if we're in the correct directory
if [ ! -f "docker-compose.yml" ]; then
    echo "❌ Please run this script from the project root directory (where docker-compose.yml is located)"
    exit 1
fi

# Start PostgreSQL and API
echo "📦 Starting PostgreSQL and API..."
make dev-up &
API_PID=$!

# Give it a moment to start
sleep 3

# Start Adminer
echo "🗄️ Starting Adminer database admin..."
docker compose up -d adminer

# Wait a bit more for services to be ready
echo "⏳ Waiting for services to be ready..."
sleep 5

# Check if API is responding
echo "🔍 Checking API health..."
if curl -s http://localhost:8081/api/v1/health > /dev/null; then
    echo "✅ API is running on http://localhost:8081"
else
    echo "⚠️ API might still be starting up..."
fi

# Check if Adminer is responding
if curl -s http://localhost:8082 > /dev/null; then
    echo "✅ Adminer is running on http://localhost:8082"
else
    echo "⚠️ Adminer might still be starting up..."
fi

echo ""
echo "🎉 Dashboard environment is ready!"
echo ""
echo "📊 Access your dashboard:"
echo "   Dashboard: file://$(pwd)/ui/dashboard.html"
echo "   API Health: http://localhost:8081/api/v1/health"
echo "   Database Admin: http://localhost:8082"
echo ""
echo "🔑 Adminer login credentials:"
echo "   Server: db"
echo "   Username: postgres"
echo "   Password: postgres"
echo "   Database: mercor"
echo ""
echo "📖 See ui/README.md for full documentation"
echo ""

# Try to open the dashboard in the default browser
if command -v open > /dev/null; then
    echo "🌐 Opening dashboard in browser..."
    open "ui/dashboard.html"
elif command -v xdg-open > /dev/null; then
    echo "🌐 Opening dashboard in browser..."
    xdg-open "ui/dashboard.html"
else
    echo "💡 Manually open ui/dashboard.html in your web browser"
fi

echo ""
echo "✋ Press Ctrl+C to stop all services when done"

# Wait for API process (this keeps the script running)
wait $API_PID 