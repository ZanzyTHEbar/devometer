#!/bin/bash
set -e

echo "🔨 Building Cracked Dev-o-Meter..."
echo ""

# Check if pnpm is installed
if ! command -v pnpm &> /dev/null; then
    echo "❌ pnpm is not installed. Please install pnpm first."
    exit 1
fi

# Check if Go is installed
if ! command -v go &> /dev/null; then
    echo "❌ Go is not installed. Please install Go first."
    exit 1
fi

echo "📦 Building frontend..."
cd frontend
pnpm install --frozen-lockfile
pnpm build

echo ""
echo "📋 Copying frontend dist to backend..."
cd ..
rm -rf backend/internal/frontend/dist
mkdir -p backend/internal/frontend
cp -r frontend/dist backend/internal/frontend/

echo ""
echo "🔧 Building backend with embedded frontend..."
cd backend
go mod download
go build -o ../bin/cracked-dev-o-meter ./cmd/server

echo ""
echo "✅ Build complete!"
echo "📍 Binary location: bin/cracked-dev-o-meter"
echo ""
echo "To run the server:"
echo "  ./bin/cracked-dev-o-meter"

