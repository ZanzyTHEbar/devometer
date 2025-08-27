export interface UserStats {
    user_id: string;
    requests_this_week: number;
    remaining_requests: number; // -1 for unlimited
    is_paid: boolean;
    week_start: string;
    week_end: string;
}

export interface AnalysisResult {
    score: number;
    confidence: number;
    posterior: number;
    contributors: Array<{
        name: string;
        contribution: number;
    }>;
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

export interface RateLimitError {
    error: string;
    message: string;
    remaining_requests: number;
    is_paid: boolean;
    week_start: string;
    week_end: string;
    upgrade_url: string;
}

export interface PaymentSession {
    session_id: string;
    url: string;
}

export interface ApiError {
    error: string;
}

export type ApiResponse = AnalysisResult | ApiError | RateLimitError;

export async function analyze(input: string, includeInLeaderboard = false): Promise<AnalysisResult> {
    if (!input.trim()) {
        throw new Error("Input cannot be empty");
    }

    try {
        const response = await fetch("http://localhost:8080/analyze", {
            method: "POST",
            headers: {
                "Content-Type": "application/json",
            },
            body: JSON.stringify({
                input: input.trim(),
                public: includeInLeaderboard
            }),
        });

        if (!response.ok) {
            const errorData = await response.json().catch(() => ({}));
            throw new Error(
                errorData.error || `HTTP ${response.status}: ${response.statusText}`
            );
        }

        const data: ApiResponse = await response.json();

        if ("error" in data) {
            throw new Error(data.error);
        }

        return data as AnalysisResult;
    } catch (error) {
        if (error instanceof Error) {
            throw error;
        }
        throw new Error("Network error occurred");
    }
}

// Health check function
export async function checkHealth(): Promise<boolean> {
    try {
        const response = await fetch("http://localhost:8080/health");
        return response.ok;
    } catch {
        return false;
    }
}

// Get user statistics
export async function getUserStats(): Promise<UserStats> {
    try {
        const response = await fetch("http://localhost:8080/user/stats");
        if (!response.ok) {
            const errorData = await response.json().catch(() => ({}));
            throw new Error(errorData.error || `HTTP ${response.status}: ${response.statusText}`);
        }
        return await response.json();
    } catch (error) {
        if (error instanceof Error) {
            throw error;
        }
        throw new Error("Network error occurred");
    }
}

// Create payment session
export async function createPaymentSession(type: "donation" | "unlimited", amount?: number): Promise<PaymentSession> {
    try {
        const response = await fetch("http://localhost:8080/payment/create-session", {
            method: "POST",
            headers: {
                "Content-Type": "application/json",
            },
            body: JSON.stringify({ type, amount }),
        });

        if (!response.ok) {
            const errorData = await response.json().catch(() => ({}));
            throw new Error(errorData.error || `HTTP ${response.status}: ${response.statusText}`);
        }

        return await response.json();
    } catch (error) {
        if (error instanceof Error) {
            throw error;
        }
        throw new Error("Network error occurred");
    }
}

// Check if response is rate limit error
export function isRateLimitError(response: ApiResponse): response is RateLimitError {
    return typeof response === 'object' && response !== null && 'remaining_requests' in response;
}

// Leaderboard types
export interface LeaderboardEntry {
    id: string;
    developer_hash: string;
    period: string;
    period_start: string;
    period_end: string;
    rank: number;
    score: number;
    confidence: number;
    input_type: string;
    is_public: boolean;
    created_at: string;
}

export interface LeaderboardResponse {
    entries: LeaderboardEntry[];
    total: number;
    period: string;
    period_start: string;
    period_end: string;
}

// Fetch leaderboard data
export async function fetchLeaderboard(period: string, limit = 50): Promise<LeaderboardResponse> {
    const response = await fetch(`/api/leaderboard/${period}?limit=${limit}`);
    if (!response.ok) {
        throw new Error(`Failed to fetch leaderboard: ${response.statusText}`);
    }
    return response.json();
}

// Fetch developer rank
export async function fetchDeveloperRank(hash: string, period: string): Promise<LeaderboardEntry> {
    const response = await fetch(`/api/leaderboard/${period}/rank/${hash}`);
    if (!response.ok) {
        throw new Error(`Failed to fetch developer rank: ${response.statusText}`);
    }
    return response.json();
}