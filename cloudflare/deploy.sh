#!/bin/bash

# Cloudflare Deployment Script for Cracked Dev-o-Meter

set -e

echo "🚀 Deploying Cracked Dev-o-Meter to Cloudflare"

# Check if wrangler is installed
if ! command -v wrangler &> /dev/null; then
    echo "❌ Wrangler CLI not found. Installing..."
    npm install -g wrangler
fi

# Check if logged in
if ! wrangler auth status &> /dev/null; then
    echo "❌ Not logged in to Cloudflare. Please run:"
    echo "   wrangler auth login"
    exit 1
fi

cd "$(dirname "$0")"

# Install dependencies
echo "📦 Installing dependencies..."
npm install

# Create D1 database
echo "🗄️  Creating D1 database..."
wrangler d1 create cracked-dev-o-meter-users

# Get the database ID and update wrangler.toml
echo "⚙️  Please update wrangler.toml with your D1 database ID"
echo "   You can find it in the output above or in your Cloudflare dashboard"

# Deploy
echo "🚀 Deploying to Cloudflare..."
wrangler deploy

echo "✅ Deployment complete!"
echo ""
echo "📋 Next steps:"
echo "1. Update your DNS to point to Cloudflare Pages"
echo "2. Set up Stripe webhooks for payment processing"
echo "3. Configure your domain in wrangler.toml"
echo "4. Test the API endpoints"
echo ""
echo "🌐 Your API will be available at:"
echo "   https://api.cracked-dev-o-meter.workers.dev"
echo ""
echo "📖 Useful commands:"
echo "   wrangler dev          # Start local development server"
echo "   wrangler tail         # View live logs"
echo "   wrangler d1 query     # Query your D1 database"
