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
}

export interface ApiError {
    error: string;
}

export type ApiResponse = AnalysisResult | ApiError;

export async function analyze(input: string): Promise<AnalysisResult> {
    if (!input.trim()) {
        throw new Error("Input cannot be empty");
    }

    try {
        const response = await fetch("http://localhost:8080/analyze", {
            method: "POST",
            headers: {
                "Content-Type": "application/json",
            },
            body: JSON.stringify({ input: input.trim() }),
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