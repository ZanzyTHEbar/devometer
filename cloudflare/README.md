# Cloudflare Deployment Guide

This guide walks you through deploying the Cracked Dev-o-Meter to Cloudflare for free/low-cost hosting with the new payment system.

## Features Enabled

✅ **5 Free Requests Per Week** - User tracking with D1 database
✅ **Stripe Payment Integration** - Donations and unlimited access
✅ **Global CDN** - Fast loading worldwide
✅ **Serverless Backend** - No server management
✅ **Free Tier** - Generous free quotas

## Prerequisites

- Cloudflare account with Workers enabled
- Domain name (optional, but recommended)
- Stripe account for payments
- Node.js 18+ and Wrangler CLI

## Quick Start

### 1. Install Wrangler CLI

```bash
npm install -g wrangler
wrangler auth login
```

### 2. Set Up D1 Database

```bash
cd cloudflare
wrangler d1 create cracked-dev-o-meter-users
```

Copy the database ID from the output and update `wrangler.toml`:

```toml
[[d1_databases]]
binding = "USER_DB"
database_name = "cracked-dev-o-meter-users"
database_id = "your_database_id_here"
```

### 3. Initialize Database Schema

```bash
# Create the tables
wrangler d1 execute cracked-dev-o-meter-users --file=../backend/internal/database/schema.sql
```

### 4. Configure Environment Variables

Update `wrangler.toml` with your values:

```toml
[vars]
JWT_SECRET = "your-super-secret-jwt-key"
STRIPE_SECRET_KEY = "sk_live_your_stripe_secret_key"
STRIPE_WEBHOOK_SECRET = "whsec_your_webhook_secret"
```

### 5. Deploy

```bash
./deploy.sh
```

## Environment Variables

| Variable                | Description                  | Required           |
| ----------------------- | ---------------------------- | ------------------ |
| `JWT_SECRET`            | Secret key for user sessions | Yes                |
| `STRIPE_SECRET_KEY`     | Your Stripe secret key       | Yes (for payments) |
| `STRIPE_WEBHOOK_SECRET` | Stripe webhook secret        | Yes (for payments) |
| `MAX_REQUESTS_PER_MIN`  | Rate limit per minute        | No (default: 30)   |
| `MAX_INPUT_LENGTH`      | Max input length             | No (default: 100)  |

## API Endpoints

### User Management

**GET /user/stats**

- Returns current user's usage statistics
- Creates user if doesn't exist

**POST /payment/create-session**

- Creates Stripe checkout session
- Supports donations and subscriptions

### Analysis

**POST /analyze**

- Main analysis endpoint
- Rate limited to 5 requests/week for free users
- Unlimited for paid users

### Health

**GET /health**

- Health check endpoint
- Returns service status

## Frontend Deployment

Deploy the frontend to Cloudflare Pages:

```bash
cd frontend
npm run build

# Deploy to Pages
wrangler pages deploy dist --name cracked-dev-o-meter
```

Update `frontend/src/api.ts` to use your Cloudflare Worker URL:

```typescript
const response = await fetch(
  "https://api.cracked-dev-o-meter.workers.dev/analyze",
  {
    // ... rest of the code
  }
);
```

## Stripe Configuration

### 1. Create Products

**Unlimited Access Subscription:**

- Product: "Unlimited Access"
- Price: $9.99/month
- API ID: `price_unlimited_monthly`

**Donations:**

- Product: "Donation"
- One-time payment
- Custom amounts allowed

### 2. Webhook Configuration

Set up webhook endpoint in Stripe:

- URL: `https://api.cracked-dev-o-meter.workers.dev/payment/webhook`
- Events: `checkout.session.completed`

### 3. Update Price IDs

Update the price IDs in `src/index.ts`:

```typescript
const priceID = "price_unlimited_monthly"; // Your actual Stripe price ID
```

## Testing

### Local Development

```bash
# Start local development server
wrangler dev

# Test API endpoints
curl http://localhost:8787/health
curl -X POST http://localhost:8787/analyze \
  -H "Content-Type: application/json" \
  -d '{"input": "torvalds"}'
```

### Production Testing

```bash
# Test health endpoint
curl https://api.cracked-dev-o-meter.workers.dev/health

# Test analysis (first 5 are free)
curl -X POST https://api.cracked-dev-o-meter.workers.dev/analyze \
  -H "Content-Type: application/json" \
  -d '{"input": "torvalds"}'

# Test user stats
curl https://api.cracked-dev-o-meter.workers.dev/user/stats
```

## Cost Estimation

### Free Tier (Generous)

- **Workers Requests**: 100,000/day free
- **D1 Database**: 500,000 reads/month free
- **Pages**: Unlimited static sites
- **Bandwidth**: 100GB/month free

### Paid Tier (if needed)

- **Extra Workers**: $0.30/100k requests
- **Extra D1**: $0.75/100k reads
- **Stripe Fees**: 2.9% + 30¢ per transaction

## Monitoring

### View Logs

```bash
# Real-time logs
wrangler tail

# Logs for specific environment
wrangler tail --env preview
```

### Database Management

```bash
# Query database
wrangler d1 query cracked-dev-o-meter-users "SELECT * FROM users LIMIT 10"

# Backup database
wrangler d1 backup create cracked-dev-o-meter-users
```

## Troubleshooting

### Common Issues

**"D1_ERROR: no such table"**

- Run the database migration script
- Check database ID in wrangler.toml

**"Rate limit exceeded"**

- Check your Cloudflare Workers usage
- Upgrade to paid plan if needed

**"Stripe webhook signature verification failed"**

- Verify STRIPE_WEBHOOK_SECRET in environment
- Check webhook endpoint URL in Stripe dashboard

### Performance Tips

- **Caching**: Use Cloudflare's edge caching for static assets
- **Database Optimization**: Add indexes for frequently queried columns
- **Rate Limiting**: Adjust limits based on your usage patterns

## Security Considerations

- **Environment Variables**: Never commit secrets to version control
- **CORS**: Configure allowed origins for your domain
- **Rate Limiting**: Protects against abuse
- **Input Validation**: Sanitizes user input
- **HTTPS**: Always use HTTPS in production

## Next Steps

1. **Custom Domain**: Set up your custom domain
2. **Monitoring**: Set up error tracking and analytics
3. **Backup**: Configure regular database backups
4. **Scaling**: Monitor usage and scale as needed

---

**🎉 Your Cracked Dev-o-Meter is now live on Cloudflare!**

Users can now:

- ✅ Get 5 free analyses per week
- ✅ Upgrade to unlimited access
- ✅ Make donations to support development
- ✅ Access the service from anywhere in the world

**Share your creation:** `https://your-domain.com`
