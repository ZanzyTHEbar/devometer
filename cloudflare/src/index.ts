import { Hono } from 'hono';
import { cors } from 'hono/cors';
import { logger } from 'hono/logger';
import { prettyJSON } from 'hono/pretty-json';

// Types
interface User {
    id: string;
    email?: string;
    ip_address: string;
    user_agent: string;
    is_paid: boolean;
    stripe_customer_id?: string;
    created_at: string;
    updated_at: string;
}

interface UserStats {
    user_id: string;
    requests_this_week: number;
    remaining_requests: number;
    is_paid: boolean;
    week_start: string;
    week_end: string;
}

interface AnalysisResult {
    score: number;
    confidence: number;
    posterior: number;
    breakdown: {
        shipping: number;
        quality: number;
        influence: number;
        complexity: number;
        collaboration: number;
        reliability: number;
        novelty: number;
    };
    user_stats?: UserStats;
}

const app = new Hono();

// Middleware
app.use('*', cors({
    origin: ['http://localhost:3000', 'http://localhost:5173', 'https://your-domain.com'],
    allowMethods: ['GET', 'POST', 'OPTIONS'],
    allowHeaders: ['Content-Type', 'Authorization'],
    credentials: true,
}));

app.use('*', logger());
app.use('*', prettyJSON());

// Rate limiting helper
const getClientIP = (c: any) => {
    return c.req.header('CF-Connecting-IP') ||
        c.req.header('X-Forwarded-For') ||
        c.req.header('X-Real-IP') ||
        'unknown';
};

// User identification helper
const getUserId = (ip: string, userAgent: string): string => {
    // Simple hash of IP + User Agent for user identification
    const crypto = await import('crypto');
    const hash = crypto.createHash('sha256');
    hash.update(ip + userAgent);
    return hash.digest('hex').substring(0, 16);
};

// Database helpers
const getOrCreateUser = async (c: any, ip: string, userAgent: string): Promise<User> => {
    const userId = getUserId(ip, userAgent);

    try {
        // Try to get existing user
        const result = await c.env.USER_DB.prepare(
            'SELECT * FROM users WHERE id = ?'
        ).bind(userId).first();

        if (result) {
            // Update last seen
            await c.env.USER_DB.prepare(
                'UPDATE users SET updated_at = ?, user_agent = ? WHERE id = ?'
            ).bind(new Date().toISOString(), userAgent, userId).run();

            return result as User;
        }
    } catch (error) {
        console.log('User not found, creating new user');
    }

    // Create new user
    const newUser: User = {
        id: userId,
        ip_address: ip,
        user_agent: userAgent,
        is_paid: false,
        created_at: new Date().toISOString(),
        updated_at: new Date().toISOString(),
    };

    await c.env.USER_DB.prepare(
        'INSERT INTO users (id, ip_address, user_agent, is_paid, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?)'
    ).bind(
        newUser.id, newUser.ip_address, newUser.user_agent,
        newUser.is_paid, newUser.created_at, newUser.updated_at
    ).run();

    return newUser;
};

const logRequest = async (c: any, userId: string, ip: string, endpoint: string, method: string, userAgent: string) => {
    const requestId = crypto.randomUUID();

    await c.env.USER_DB.prepare(
        'INSERT INTO request_logs (id, user_id, ip_address, endpoint, method, user_agent, created_at) VALUES (?, ?, ?, ?, ?, ?, ?)'
    ).bind(
        requestId, userId, ip, endpoint, method, userAgent, new Date().toISOString()
    ).run();
};

const getWeeklyUsage = async (c: any, userId: string): Promise<UserStats> => {
    const now = new Date();
    const weekStart = new Date(now);
    weekStart.setDate(now.getDate() - now.getDay()); // Start of week (Sunday)
    weekStart.setHours(0, 0, 0, 0);

    const weekEnd = new Date(weekStart);
    weekEnd.setDate(weekStart.getDate() + 7);

    // Get user payment status
    const userResult = await c.env.USER_DB.prepare(
        'SELECT is_paid FROM users WHERE id = ?'
    ).bind(userId).first();

    const isPaid = userResult?.is_paid || false;

    // Count requests this week
    const requestsResult = await c.env.USER_DB.prepare(
        'SELECT COUNT(*) as count FROM request_logs WHERE user_id = ? AND created_at >= ? AND created_at < ?'
    ).bind(userId, weekStart.toISOString(), weekEnd.toISOString()).first();

    const requestsThisWeek = requestsResult?.count || 0;
    const freeLimit = 5;
    const remainingRequests = isPaid ? -1 : Math.max(0, freeLimit - requestsThisWeek);

    return {
        user_id: userId,
        requests_this_week: requestsThisWeek,
        remaining_requests: remainingRequests,
        is_paid: isPaid,
        week_start: weekStart.toISOString().split('T')[0],
        week_end: weekEnd.toISOString().split('T')[0],
    };
};

const canMakeRequest = async (c: any, userId: string): Promise<boolean> => {
    const usage = await getWeeklyUsage(c, userId);
    return usage.remaining_requests === -1 || usage.remaining_requests > 0;
};

// Routes
app.get('/health', (c) => {
    return c.json({
        status: 'ok',
        timestamp: new Date().toISOString(),
        version: '1.0.0-cloudflare',
        services: {
            'cloudflare-d1': 'operational',
            'cloudflare-kv': 'operational',
        }
    });
});

app.get('/user/stats', async (c) => {
    const ip = getClientIP(c);
    const userAgent = c.req.header('User-Agent') || 'unknown';

    try {
        const user = await getOrCreateUser(c, ip, userAgent);
        const stats = await getWeeklyUsage(c, user.id);

        return c.json(stats);
    } catch (error) {
        console.error('Error getting user stats:', error);
        return c.json({ error: 'Failed to get user stats' }, 500);
    }
});

app.post('/analyze', async (c) => {
    const ip = getClientIP(c);
    const userAgent = c.req.header('User-Agent') || 'unknown';

    try {
        const body = await c.req.json();
        const input = body.input;

        if (!input || typeof input !== 'string' || input.trim().length === 0) {
            return c.json({ error: 'Input is required' }, 400);
        }

        // Get or create user
        const user = await getOrCreateUser(c, ip, userAgent);

        // Check if user can make request
        const canRequest = await canMakeRequest(c, user.id);
        if (!canRequest) {
            const usage = await getWeeklyUsage(c, user.id);
            return c.json({
                error: 'Weekly request limit exceeded',
                message: 'You\'ve used all 5 free requests this week',
                remaining_requests: usage.remaining_requests,
                is_paid: usage.is_paid,
                week_start: usage.week_start,
                week_end: usage.week_end,
                upgrade_url: '/upgrade',
            }, 429);
        }

        // Log the request
        await logRequest(c, user.id, ip, '/analyze', 'POST', userAgent);

        // Simple mock analysis (replace with real GitHub/X API calls)
        const mockResult: AnalysisResult = {
            score: Math.floor(Math.random() * 40) + 30, // 30-70 range
            confidence: 0.8 + Math.random() * 0.15, // 0.8-0.95 range
            posterior: 0.85 + Math.random() * 0.1, // 0.85-0.95 range
            breakdown: {
                shipping: 0.7 + Math.random() * 0.3,
                quality: 0.6 + Math.random() * 0.4,
                influence: 0.5 + Math.random() * 0.5,
                complexity: 0.4 + Math.random() * 0.6,
                collaboration: 0.55 + Math.random() * 0.45,
                reliability: 0.65 + Math.random() * 0.35,
                novelty: 0.35 + Math.random() * 0.65,
            }
        };

        // Include user stats in response
        const usage = await getWeeklyUsage(c, user.id);
        mockResult.user_stats = usage;

        return c.json(mockResult);

    } catch (error) {
        console.error('Analysis error:', error);
        return c.json({ error: 'Analysis failed' }, 500);
    }
});

app.post('/payment/create-session', async (c) => {
    // This would integrate with Stripe
    // For now, return a mock response
    return c.json({
        error: 'Payment system not yet configured for Cloudflare deployment',
        message: 'Please use the Docker deployment for full payment functionality'
    }, 501);
});

export default app;
