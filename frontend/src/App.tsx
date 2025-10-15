import {
  createSignal,
  createResource,
  Show,
  For,
  type Component,
  ErrorBoundary,
  onMount,
} from "solid-js";
import Meter from "./components/Meter";
import UserStats from "./components/UserStats";
import Leaderboard from "./components/Leaderboard";
import LeaderboardOptInModal from "./components/LeaderboardOptInModal";
import LeaderboardWidget from "./components/LeaderboardWidget";
import { analyze, optInToLeaderboard, type AnalysisResult, isRateLimitError } from "./api";

// Browser API declarations for TypeScript
declare const URLSearchParams: typeof globalThis.URLSearchParams;
declare const navigator: typeof globalThis.navigator;

type InputMode = "github-user" | "github-repo" | "x-account" | "combined";

interface ValidationState {
  isValid: boolean;
  message?: string;
}

const App: Component = () => {
  const [input, setInput] = createSignal("");
  const [trigger, setTrigger] = createSignal(0);
  const [error, setError] = createSignal<string | null>(null);
  const [inputMode, setInputMode] = createSignal<InputMode>("github-repo");
  const [validation, setValidation] = createSignal<ValidationState>({ isValid: true });
  const [showPaymentModal, setShowPaymentModal] = createSignal(false);
  const [userStatsRefreshTrigger, setUserStatsRefreshTrigger] = createSignal(0);
  const [activeTab, setActiveTab] = createSignal<"analysis" | "leaderboard">("analysis");
  const [includeInLeaderboard, setIncludeInLeaderboard] = createSignal(false);
  const [showLeaderboardModal, setShowLeaderboardModal] = createSignal(false);
  const [currentDeveloperHash, setCurrentDeveloperHash] = createSignal<string | null>(null);
  const [currentAnalysisResult, setCurrentAnalysisResult] = createSignal<AnalysisResult | null>(
    null
  );

  // Input mode configurations
  const inputModes = [
    {
      id: "github-repo" as InputMode,
      label: "GitHub Repo",
      placeholder: "e.g., facebook/react, microsoft/vscode",
      icon: "📦",
      examples: ["facebook/react", "microsoft/vscode", "torvalds/linux"],
    },
    {
      id: "github-user" as InputMode,
      label: "GitHub User",
      placeholder: "e.g., torvalds, gaearon",
      icon: "👤",
      examples: ["torvalds", "gaearon", "tj"],
    },
    {
      id: "x-account" as InputMode,
      label: "X Account",
      placeholder: "e.g., @vercel, @github",
      icon: "🐦",
      examples: ["@vercel", "@github", "@reactjs"],
    },
    {
      id: "combined" as InputMode,
      label: "Combined",
      placeholder: "e.g., github:torvalds x:@elonmusk",
      icon: "🔗",
      examples: [
        "github:torvalds x:@elonmusk",
        "github:gaearon x:@reactjs",
        "github:tj x:@tjholowaychuk",
      ],
    },
  ];

  // Get current mode config
  const currentMode = () => inputModes.find((mode) => mode.id === inputMode());

  // Validation function
  const validateInput = (value: string, mode: InputMode): ValidationState => {
    if (!value.trim()) {
      return { isValid: false, message: "Please enter a value" };
    }

    switch (mode) {
      case "github-repo": {
        if (!value.includes("/")) {
          return { isValid: false, message: "Use format: owner/repo (e.g., facebook/react)" };
        }
        const repoParts = value.split("/");
        if (repoParts.length !== 2 || repoParts[0].trim() === "" || repoParts[1].trim() === "") {
          return { isValid: false, message: "Invalid repository format" };
        }
        break;
      }
      case "github-user":
        if (value.includes("/") || value.includes("@")) {
          return { isValid: false, message: "Enter just the username (no @ or /)" };
        }
        break;
      case "x-account":
        if (!value.startsWith("@")) {
          return { isValid: false, message: "X accounts must start with @" };
        }
        if (value.length < 2) {
          return { isValid: false, message: "Please enter a valid X account" };
        }
        break;
      case "combined": {
        // Validate combined format: "github:username x:@account"
        const githubMatch = value.match(/github:([^\s]+)/);
        const xMatch = value.match(/x:([^\s]+)/);

        if (!githubMatch) {
          return { isValid: false, message: "Include GitHub part: github:username" };
        }
        if (!xMatch) {
          return { isValid: false, message: "Include X part: x:@account" };
        }

        const githubUser = githubMatch[1];
        const xAccount = xMatch[1];

        if (!githubUser || githubUser.includes("/") || githubUser.includes("@")) {
          return { isValid: false, message: "Invalid GitHub username format" };
        }
        if (!xAccount || !xAccount.startsWith("@") || xAccount.length < 2) {
          return { isValid: false, message: "Invalid X account format (must start with @)" };
        }
        break;
      }
    }

    return { isValid: true };
  };

  // Handle input changes with validation
  const handleInputChange = (value: string) => {
    setInput(value);
    const result = validateInput(value, inputMode());
    setValidation(result);
  };

  const [analysisResult] = createResource(
    () => trigger(), // Only depend on trigger, not input
    // eslint-disable-next-line solid/reactivity
    async (triggerValue) => {
      if (triggerValue === 0) return null; // Don't run on initial load
      const query = input();
      if (!query.trim()) return null;

      try {
        setError(null); // Clear any previous errors
        const result = await analyze(query, includeInLeaderboard());

        // Check if this is a rate limit error (shouldn't happen if middleware works, but just in case)
        if (isRateLimitError(result)) {
          setError(result.message);
          setShowPaymentModal(true);
          return null;
        }

        // Store analysis result and developer hash for opt-in modal
        setCurrentAnalysisResult(result);
        if (result.developer_hash) {
          setCurrentDeveloperHash(result.developer_hash);
          // Show modal after short delay for better UX
          setTimeout(() => setShowLeaderboardModal(true), 1000);
        }

        return result;
      } catch (err) {
        const errorMessage = err instanceof Error ? err.message : "An unknown error occurred";
        setError(errorMessage);

        // Check if it's a rate limit error from the API
        if (typeof err === "object" && err !== null && "remaining_requests" in err) {
          const rateLimitErr = err as any;
          setError(rateLimitErr.message || "Weekly request limit exceeded");
          setShowPaymentModal(true);
        }

        throw err; // Re-throw to be caught by ErrorBoundary if needed
      }
    }
  );

  const handleAnalyze = () => {
    const currentInput = input();
    const currentValidation = validateInput(currentInput, inputMode());

    if (!currentValidation.isValid) {
      setValidation(currentValidation);
      return;
    }

    if (currentInput.trim()) {
      setError(null); // Clear any previous errors
      setTrigger((prev) => prev + 1);
    }
  };

  const handleKeyPress = (e: KeyboardEvent) => {
    if (e.key === "Enter") {
      handleAnalyze();
    }
  };

  // Handle leaderboard opt-in
  const handleLeaderboardOptIn = async (optIn: boolean, displayName: string) => {
    if (!currentDeveloperHash()) return;

    try {
      await optInToLeaderboard(currentDeveloperHash()!, optIn, displayName);
      setShowLeaderboardModal(false);

      // Show success message or navigate to leaderboard
      if (optIn) {
        // Optionally navigate to leaderboard tab
        // setActiveTab("leaderboard");
      }
    } catch (error) {
      console.error("Failed to update opt-in status:", error);
      throw error;
    }
  };

  // Listen for leaderboard navigation events from widget
  onMount(() => {
    const handleNavigateLeaderboard = () => {
      setActiveTab("leaderboard");
    };
    window.addEventListener("navigate-leaderboard", handleNavigateLeaderboard);
    return () => window.removeEventListener("navigate-leaderboard", handleNavigateLeaderboard);
  });

  const handleTryAgain = () => {
    setError(null); // Clear error state
    setTrigger((prev) => prev + 1); // Retry the analysis
  };

  const handleModeChange = (mode: InputMode) => {
    setInputMode(mode);
    setValidation({ isValid: true }); // Reset validation when switching modes
    // Re-validate current input with new mode
    if (input().trim()) {
      const result = validateInput(input(), mode);
      setValidation(result);
    }
  };

  // Gauge range labels based on score
  const getGaugeLabel = (score: number) => {
    if (score < 30) return { label: "Chill Dev", color: "text-green-600", bgColor: "bg-green-50" };
    if (score < 70) return { label: "Balanced", color: "text-yellow-600", bgColor: "bg-yellow-50" };
    return { label: "Cracked Out", color: "text-red-600", bgColor: "bg-red-50" };
  };

  // Generate shareable link
  const generateShareLink = () => {
    const baseUrl = window.location.origin;
    const params = new URLSearchParams({
      mode: inputMode(),
      input: input(),
      score: analysisResult()?.score.toString() || "50",
    });
    return `${baseUrl}?${params.toString()}`;
  };

  const copyToClipboard = async (text: string) => {
    try {
      await navigator.clipboard.writeText(text);
      // Could add a toast notification here
    } catch (err) {
      console.error("Failed to copy:", err);
    }
  };

  return (
    <ErrorBoundary
      fallback={(err, reset) => (
        <div class="min-h-screen flex items-center justify-center bg-slate-50 p-4">
          <div class="text-center max-w-md">
            <h1 class="text-2xl font-bold text-red-600 mb-4">⚠️ Something went wrong</h1>
            <p class="text-red-500 mb-6">{err.message}</p>
            <div class="space-y-3">
              <button
                onClick={() => {
                  setError(null);
                  reset();
                }}
                class="w-full px-4 py-2 bg-indigo-600 text-white rounded-lg hover:bg-indigo-700 transition-colors"
              >
                Try Again
              </button>
              <button
                onClick={() => {
                  setInput("");
                  setError(null);
                  setTrigger(0);
                  setValidation({ isValid: true });
                  reset();
                }}
                class="w-full px-4 py-2 bg-gray-600 text-white rounded-lg hover:bg-gray-700 transition-colors"
              >
                Start Over
              </button>
            </div>
          </div>
        </div>
      )}
    >
      <div class="min-h-screen bg-gradient-to-br from-slate-50 to-indigo-50 p-4">
        <div class="max-w-4xl mx-auto">
          {/* Header */}
          <div class="text-center mb-8">
            <h1 class="text-4xl font-bold mb-3 bg-gradient-to-r from-indigo-600 to-purple-600 bg-clip-text text-transparent">
              🔍 Cracked Dev-o-Meter
            </h1>
            <p class="text-xl text-slate-600 mb-2">
              Discover how cracked your development game really is! 🚀
            </p>
            <p class="text-slate-500">
              Analyze GitHub repos, users, X accounts, or combine multiple sources
            </p>
          </div>

          {/* Main Tabs */}
          <div class="flex justify-center mb-8">
            <div class="tabs tabs-boxed bg-white/80 backdrop-blur-sm">
              <button
                class={`tab ${activeTab() === "analysis" ? "tab-active" : ""}`}
                onClick={() => setActiveTab("analysis")}
              >
                🔍 Analysis
              </button>
              <button
                class={`tab ${activeTab() === "leaderboard" ? "tab-active" : ""}`}
                onClick={() => setActiveTab("leaderboard")}
              >
                🏆 Leaderboard
              </button>
            </div>
          </div>

          {/* Analysis Tab Content */}
          <Show when={activeTab() === "analysis"}>
            {/* Leaderboard Widget */}
            <div class="mb-8">
              <LeaderboardWidget className="max-w-md mx-auto" />
            </div>

            {/* Input Section */}
            <div class="bg-white/80 backdrop-blur-sm rounded-2xl shadow-xl border border-white/20 p-8 mb-8">
              {/* Input Mode Tabs */}
              <div class="grid grid-cols-2 md:flex md:space-x-1 mb-6 bg-slate-100 p-1 rounded-lg">
                <For each={inputModes}>
                  {(mode) => (
                    <button
                      onClick={() => handleModeChange(mode.id)}
                      class={`flex-1 py-2 px-4 rounded-md text-sm font-medium transition-all ${
                        inputMode() === mode.id
                          ? "bg-white shadow-sm text-indigo-700"
                          : "text-slate-600 hover:text-slate-800"
                      }`}
                      aria-label={`Switch to ${mode.label} input mode`}
                    >
                      <span class="mr-2">{mode.icon}</span>
                      <span class="hidden sm:inline">{mode.label}</span>
                      <span class="sm:hidden">
                        {mode.label === "Combined" ? "Combo" : mode.label}
                      </span>
                    </button>
                  )}
                </For>
              </div>

              {/* Input Field */}
              <div class="space-y-3">
                <div class="relative">
                  <input
                    type="text"
                    class={`w-full p-4 rounded-xl border-2 transition-all text-lg ${
                      !validation().isValid
                        ? "border-red-300 focus:border-red-500 bg-red-50"
                        : "border-slate-200 focus:border-indigo-500 bg-white"
                    }`}
                    placeholder={currentMode()?.placeholder}
                    value={input()}
                    onInput={(e) => handleInputChange((e.target as HTMLInputElement).value)}
                    onKeyPress={handleKeyPress}
                    disabled={analysisResult.loading}
                    aria-label={`${currentMode()?.label} input`}
                    aria-invalid={!validation().isValid}
                    autocomplete="off"
                  />

                  {/* Validation Error */}
                  <Show when={!validation().isValid && validation().message}>
                    <p class="absolute left-0 top-full mt-2 text-sm text-red-600" role="alert">
                      {validation().message}
                    </p>
                  </Show>
                </div>

                {/* Examples */}
                <div class="space-y-2">
                  <div class="flex items-center gap-2">
                    <span class="text-sm text-slate-500">Try:</span>
                    <For each={currentMode()?.examples.slice(0, 2)}>
                      {(example) => (
                        <button
                          onClick={() => handleInputChange(example)}
                          class="px-3 py-1 text-sm bg-slate-100 hover:bg-slate-200 text-slate-700 rounded-full transition-colors"
                          aria-label={`Use example: ${example}`}
                        >
                          {example}
                        </button>
                      )}
                    </For>
                  </div>
                  {currentMode()?.id === "combined" && (
                    <p class="text-xs text-slate-400">Format: github:username x:@accountname</p>
                  )}
                </div>

                {/* Privacy Consent */}
                <div class="flex items-start space-x-3 p-3 bg-slate-50 rounded-lg border border-slate-200">
                  <input
                    type="checkbox"
                    id="leaderboard-consent"
                    checked={includeInLeaderboard()}
                    onChange={(e) =>
                      setIncludeInLeaderboard((e.target as HTMLInputElement).checked)
                    }
                    class="checkbox checkbox-primary mt-0.5"
                  />
                  <div class="flex-1">
                    <label
                      for="leaderboard-consent"
                      class="text-sm font-medium text-slate-700 cursor-pointer"
                    >
                      Include in Public Leaderboard 🏆
                    </label>
                    <p class="text-xs text-slate-500 mt-1">
                      Opt-in to have your anonymized analysis appear on the public leaderboard. You
                      can change this setting anytime.
                      <a
                        href="/privacy-policy"
                        target="_blank"
                        class="text-primary hover:underline ml-1"
                      >
                        Learn more →
                      </a>
                    </p>
                  </div>
                </div>

                {/* Analyze Button */}
                <button
                  onClick={handleAnalyze}
                  disabled={analysisResult.loading || !input().trim() || !validation().isValid}
                  class="w-full py-4 bg-gradient-to-r from-indigo-600 to-purple-600 text-white rounded-xl font-semibold text-lg hover:from-indigo-700 hover:to-purple-700 transition-all disabled:opacity-50 disabled:cursor-not-allowed focus:outline-none focus:ring-4 focus:ring-indigo-300"
                  aria-label={analysisResult.loading ? "Analyzing..." : "Analyze for crackedness"}
                >
                  {analysisResult.loading ? (
                    <span class="flex items-center justify-center gap-3">
                      <svg class="animate-spin h-5 w-5" viewBox="0 0 24 24">
                        <circle
                          class="opacity-25"
                          cx="12"
                          cy="12"
                          r="10"
                          stroke="currentColor"
                          stroke-width="4"
                          fill="none"
                        />
                        <path
                          class="opacity-75"
                          fill="currentColor"
                          d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"
                        />
                      </svg>
                      Analyzing your crackedness...
                    </span>
                  ) : (
                    <span class="flex items-center justify-center gap-2">
                      🔍 Analyze Crackedness
                    </span>
                  )}
                </button>
              </div>
            </div>

            {/* User Stats */}
            <div class="mb-8">
              <UserStats
                onPaymentSuccess={() => {
                  setUserStatsRefreshTrigger((prev) => prev + 1);
                  setError(null); // Clear any rate limit errors
                }}
              />
            </div>

            {/* Error Display */}
            <Show when={error()}>
              <div class="bg-red-50 border border-red-200 rounded-xl p-4 mb-6">
                <div class="flex items-start justify-between">
                  <div class="flex-1">
                    <h3 class="font-semibold text-red-800 mb-2">
                      {error()?.includes("limit") ? "Request Limit Reached" : "Analysis Failed"}
                    </h3>
                    <p class="text-red-700">{error()}</p>
                    {error()?.includes("limit") && (
                      <p class="text-sm text-red-600 mt-2">
                        💡 Upgrade to unlimited access or wait for the weekly reset!
                      </p>
                    )}
                  </div>
                  <div class="flex gap-2 ml-4">
                    {error()?.includes("limit") && (
                      <button
                        onClick={() => setShowPaymentModal(true)}
                        class="px-4 py-2 bg-green-600 text-white rounded-lg hover:bg-green-700 transition-colors text-sm"
                      >
                        Upgrade
                      </button>
                    )}
                    <button
                      onClick={handleTryAgain}
                      class="px-4 py-2 bg-red-600 text-white rounded-lg hover:bg-red-700 transition-colors text-sm"
                    >
                      Retry
                    </button>
                  </div>
                </div>
              </div>
            </Show>

            {/* Results Section */}
            <Show
              when={analysisResult()}
              keyed
              fallback={
                <div class="text-center py-16">
                  <div class="text-6xl mb-4">🎯</div>
                  <h3 class="text-xl font-semibold text-slate-700 mb-2">Ready to Analyze</h3>
                  <p class="text-slate-500">
                    Enter your details above to discover your developer crackedness!
                  </p>
                </div>
              }
            >
              {(result: AnalysisResult) => {
                const gaugeInfo = getGaugeLabel(result.score);

                return (
                  <div class="space-y-8">
                    {/* Main Score Display */}
                    <div class="bg-white/80 backdrop-blur-sm rounded-2xl shadow-xl border border-white/20 p-8">
                      <div class="text-center mb-8">
                        <div class="inline-flex items-center gap-4 mb-4">
                          <span class="text-4xl">
                            {gaugeInfo.label === "Cracked Out"
                              ? "🔥"
                              : gaugeInfo.label === "Balanced"
                                ? "⚖️"
                                : "😌"}
                          </span>
                          <div class={`px-4 py-2 rounded-full ${gaugeInfo.bgColor}`}>
                            <span class={`font-semibold ${gaugeInfo.color}`}>
                              {gaugeInfo.label}
                            </span>
                          </div>
                        </div>

                        <div class="flex justify-center mb-4">
                          <div style={{ width: "320px", height: "160px" }}>
                            <Meter value={result.score} animated />
                          </div>
                        </div>

                        <div class="text-6xl font-bold bg-gradient-to-r from-indigo-600 to-purple-600 bg-clip-text text-transparent mb-2">
                          {result.score}/100
                        </div>
                        <p class="text-slate-600">
                          Confidence: {(result.confidence * 100).toFixed(1)}%
                        </p>
                      </div>

                      {/* Share Section */}
                      <div class="border-t border-slate-200 pt-6">
                        <h3 class="font-semibold text-slate-800 mb-3 text-center">
                          Share Your Results
                        </h3>
                        <div class="flex gap-3 justify-center">
                          <button
                            onClick={() => copyToClipboard(generateShareLink())}
                            class="px-4 py-2 bg-indigo-100 hover:bg-indigo-200 text-indigo-700 rounded-lg transition-colors text-sm"
                          >
                            📋 Copy Link
                          </button>
                          <button
                            onClick={() =>
                              copyToClipboard(
                                `Just got a ${result.score}% crackedness score! ${generateShareLink()}`
                              )
                            }
                            class="px-4 py-2 bg-blue-100 hover:bg-blue-200 text-blue-700 rounded-lg transition-colors text-sm"
                          >
                            🐦 Tweet It
                          </button>
                        </div>
                      </div>
                    </div>

                    {/* Detailed Breakdown */}
                    <div class="grid grid-cols-1 lg:grid-cols-2 gap-8">
                      {/* Category Breakdown */}
                      <div class="bg-white/80 backdrop-blur-sm rounded-2xl shadow-xl border border-white/20 p-6">
                        <h3 class="font-semibold text-slate-800 mb-4 flex items-center gap-2">
                          📊 Category Breakdown
                        </h3>
                        <div class="space-y-4">
                          <div class="flex justify-between items-center p-3 bg-green-50 rounded-lg">
                            <span class="font-medium text-green-800">🚀 Shipping</span>
                            <span class="text-xl font-bold text-green-600">
                              {(result.breakdown.shipping * 100).toFixed(0)}%
                            </span>
                          </div>
                          <div class="flex justify-between items-center p-3 bg-blue-50 rounded-lg">
                            <span class="font-medium text-blue-800">✨ Quality</span>
                            <span class="text-xl font-bold text-blue-600">
                              {(result.breakdown.quality * 100).toFixed(0)}%
                            </span>
                          </div>
                          <div class="flex justify-between items-center p-3 bg-purple-50 rounded-lg">
                            <span class="font-medium text-purple-800">🌟 Influence</span>
                            <span class="text-xl font-bold text-purple-600">
                              {(result.breakdown.influence * 100).toFixed(0)}%
                            </span>
                          </div>
                          <div class="flex justify-between items-center p-3 bg-orange-50 rounded-lg">
                            <span class="font-medium text-orange-800">🧩 Complexity</span>
                            <span class="text-xl font-bold text-orange-600">
                              {(result.breakdown.complexity * 100).toFixed(0)}%
                            </span>
                          </div>
                        </div>
                      </div>

                      {/* Key Insights */}
                      <div class="bg-white/80 backdrop-blur-sm rounded-2xl shadow-xl border border-white/20 p-6">
                        <h3 class="font-semibold text-slate-800 mb-4 flex items-center gap-2">
                          💡 Key Insights
                        </h3>
                        <div class="space-y-4">
                          <div class="p-4 bg-gradient-to-r from-indigo-50 to-purple-50 rounded-lg">
                            <h4 class="font-semibold text-indigo-800 mb-2">
                              🎯 Overall Assessment
                            </h4>
                            <p class="text-indigo-700 text-sm">
                              {result.score < 30
                                ? "You're taking it easy! Focus on steady progress and quality work."
                                : result.score < 70
                                  ? "Balanced approach! You're hitting good velocity without sacrificing quality."
                                  : "You're absolutely cracked! High velocity, influence, and complexity - keep pushing!"}
                            </p>
                          </div>

                          <Show when={result.contributors && result.contributors.length > 0}>
                            <div class="p-4 bg-gradient-to-r from-green-50 to-blue-50 rounded-lg">
                              <h4 class="font-semibold text-green-800 mb-3">🏆 Top Contributors</h4>
                              <div class="space-y-2">
                                <For each={result.contributors.slice(0, 3)}>
                                  {(contributor) => (
                                    <div class="flex justify-between items-center text-sm">
                                      <span class="text-slate-700">{contributor.name}</span>
                                      <span class="font-medium text-green-600">
                                        +{(contributor.contribution * 100).toFixed(1)}pts
                                      </span>
                                    </div>
                                  )}
                                </For>
                              </div>
                            </div>
                          </Show>
                        </div>
                      </div>
                    </div>
                  </div>
                );
              }}
            </Show>
          </Show>

          {/* Leaderboard Tab Content */}
          <Show when={activeTab() === "leaderboard"}>
            <Leaderboard />
          </Show>
        </div>

        {/* Leaderboard Opt-in Modal */}
        <LeaderboardOptInModal
          isOpen={showLeaderboardModal()}
          onClose={() => setShowLeaderboardModal(false)}
          onOptIn={handleLeaderboardOptIn}
          developerHash={currentDeveloperHash() || ""}
          score={currentAnalysisResult()?.score || 0}
          githubUsername={undefined}
          xUsername={undefined}
        />
      </div>
    </ErrorBoundary>
  );
};

export default App;
