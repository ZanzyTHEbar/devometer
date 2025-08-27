import { createSignal, createResource, Show, For, type Component, ErrorBoundary } from "solid-js";
import Meter from "./components/Meter";
import { analyze, type AnalysisResult } from "./api";

const App: Component = () => {
  const [input, setInput] = createSignal("");
  const [trigger, setTrigger] = createSignal(0);

  const [analysisResult] = createResource(
    () => [input(), trigger()] as const,
    async ([query]) => {
      if (!query.trim()) return null;
      return await analyze(query);
    }
  );

  const handleAnalyze = () => {
    const currentInput = input();
    if (currentInput.trim()) {
      setTrigger((prev) => prev + 1);
    }
  };

  const handleKeyPress = (e: KeyboardEvent) => {
    if (e.key === "Enter") {
      handleAnalyze();
    }
  };

  return (
    <ErrorBoundary
      fallback={(err, reset) => (
        <div class="min-h-screen flex items-center justify-center bg-slate-50 p-4">
          <div class="text-center">
            <h1 class="text-2xl font-bold text-red-600 mb-4">Something went wrong</h1>
            <p class="text-red-500 mb-4">{err.message}</p>
            <button
              onClick={reset}
              class="px-4 py-2 bg-indigo-600 text-white rounded-lg hover:bg-indigo-700"
            >
              Try Again
            </button>
          </div>
        </div>
      )}
    >
      <div class="min-h-screen flex items-center justify-center bg-slate-50 p-4">
        <div class="w-full max-w-2xl p-8 rounded-xl shadow-lg glass">
          <h1 class="text-3xl font-bold mb-2 bg-gradient-to-r from-indigo-600 to-purple-600 bg-clip-text text-transparent">
            Cracked Dev-o-Meter
          </h1>
          <p class="text-slate-600 mb-8">
            Enter a GitHub username, repository, or X account to evaluate developer crackedness
          </p>

          <div class="flex flex-col sm:flex-row gap-4 mb-8">
            <input
              type="text"
              class="flex-1 p-3 rounded-lg border border-slate-200 focus:border-indigo-500 focus:ring-2 focus:ring-indigo-200 transition-all"
              placeholder="e.g., github.com/user/repo or @username"
              value={input()}
              onInput={(e) => setInput((e.target as HTMLInputElement).value)}
              onKeyPress={handleKeyPress}
              disabled={analysisResult.loading}
            />
            <button
              class="px-6 py-3 bg-gradient-to-r from-indigo-600 to-purple-600 text-white rounded-lg font-medium hover:from-indigo-700 hover:to-purple-700 transition-all disabled:opacity-50 disabled:cursor-not-allowed"
              onClick={handleAnalyze}
              disabled={analysisResult.loading || !input().trim()}
            >
              {analysisResult.loading ? (
                <span class="flex items-center gap-2">
                  <svg class="animate-spin h-4 w-4" viewBox="0 0 24 24">
                    <circle
                      class="opacity-25"
                      cx="12"
                      cy="12"
                      r="10"
                      stroke="currentColor"
                      stroke-width="4"
                      fill="none"
                    ></circle>
                    <path
                      class="opacity-75"
                      fill="currentColor"
                      d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"
                    ></path>
                  </svg>
                  Analyzing...
                </span>
              ) : (
                "Analyze"
              )}
            </button>
          </div>

          <div class="space-y-6">
            {/* Meter Display */}
            <div class="bg-white/50 rounded-xl border border-white/20 p-6 backdrop-blur-sm">
              <div class="flex justify-center">
                <div style={{ width: "320px", height: "160px" }}>
                  <Meter value={analysisResult()?.score || 50} animated />
                </div>
              </div>
            </div>

            {/* Results Display */}
            <Show
              when={analysisResult()}
              fallback={
                <div class="text-center text-slate-500 py-8">
                  Enter a GitHub repo or username above to see the crackedness analysis
                </div>
              }
              keyed
            >
              {(result: AnalysisResult) => (
                <div class="space-y-4">
                  <div class="grid grid-cols-1 md:grid-cols-2 gap-4">
                    {/* Score Summary */}
                    <div class="p-4 bg-green-50 border border-green-200 rounded-lg">
                      <h3 class="font-semibold text-green-800 mb-2">Crackedness Score</h3>
                      <div class="text-3xl font-bold text-green-600">{result.score}/100</div>
                      <div class="text-sm text-green-600 mt-1">
                        Confidence: {(result.confidence * 100).toFixed(1)}%
                      </div>
                    </div>

                    {/* Category Breakdown */}
                    <div class="p-4 bg-blue-50 border border-blue-200 rounded-lg">
                      <h3 class="font-semibold text-blue-800 mb-3">Category Breakdown</h3>
                      <div class="space-y-2 text-sm">
                        <div class="flex justify-between">
                          <span>Shipping:</span>
                          <span class="font-medium">
                            {(result.breakdown.shipping * 100).toFixed(0)}%
                          </span>
                        </div>
                        <div class="flex justify-between">
                          <span>Quality:</span>
                          <span class="font-medium">
                            {(result.breakdown.quality * 100).toFixed(0)}%
                          </span>
                        </div>
                        <div class="flex justify-between">
                          <span>Influence:</span>
                          <span class="font-medium">
                            {(result.breakdown.influence * 100).toFixed(0)}%
                          </span>
                        </div>
                        <div class="flex justify-between">
                          <span>Complexity:</span>
                          <span class="font-medium">
                            {(result.breakdown.complexity * 100).toFixed(0)}%
                          </span>
                        </div>
                      </div>
                    </div>
                  </div>

                  {/* Top Contributors */}
                  <Show when={result.contributors && result.contributors.length > 0}>
                    <div class="p-4 bg-purple-50 border border-purple-200 rounded-lg">
                      <h3 class="font-semibold text-purple-800 mb-3">Top Contributors</h3>
                      <div class="space-y-2">
                        <For each={result.contributors.slice(0, 3)}>
                          {(contributor) => (
                            <div class="flex justify-between items-center">
                              <span class="text-sm text-purple-700">{contributor.name}</span>
                              <span class="text-sm font-medium text-purple-600">
                                +{(contributor.contribution * 100).toFixed(1)}pts
                              </span>
                            </div>
                          )}
                        </For>
                      </div>
                    </div>
                  </Show>
                </div>
              )}
            </Show>
          </div>
        </div>
      </div>
    </ErrorBoundary>
  );
};

export default App;
