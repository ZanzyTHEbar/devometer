import { createSignal, createResource, onMount, For, Show } from "solid-js";
import { LeaderboardResponse, LeaderboardEntry } from "../api";

interface LeaderboardProps {
  className?: string;
}

const PERIOD_OPTIONS = [
  { value: "daily", label: "Today", description: "Top developers today" },
  { value: "weekly", label: "This Week", description: "Top developers this week" },
  { value: "monthly", label: "This Month", description: "Top developers this month" },
  { value: "all_time", label: "All Time", description: "All-time top developers" },
];

const LIMIT_OPTIONS = [
  { value: 25, label: "Top 25" },
  { value: 50, label: "Top 50" },
  { value: 100, label: "Top 100" },
];

export default function Leaderboard(props: LeaderboardProps) {
  const [selectedPeriod, setSelectedPeriod] = createSignal("weekly");
  const [selectedLimit, setSelectedLimit] = createSignal(50);
  const [isLoading, setIsLoading] = createSignal(false);
  const [error, setError] = createSignal<string | null>(null);

  // Fetch leaderboard data
  const fetchLeaderboard = async (period: string, limit: number): Promise<LeaderboardResponse> => {
    const response = await fetch(`/api/leaderboard/${period}?limit=${limit}`);
    if (!response.ok) {
      throw new Error(`Failed to fetch leaderboard: ${response.statusText}`);
    }
    return response.json();
  };

  const [leaderboardData, { refetch }] = createResource(
    () => ({ period: selectedPeriod(), limit: selectedLimit() }),
    async ({ period, limit }) => {
      setIsLoading(true);
      setError(null);
      try {
        return await fetchLeaderboard(period, limit);
      } catch (err) {
        setError(err instanceof Error ? err.message : "Unknown error occurred");
        throw err;
      } finally {
        setIsLoading(false);
      }
    }
  );

  const handlePeriodChange = (period: string) => {
    setSelectedPeriod(period);
    refetch();
  };

  const handleLimitChange = (limit: number) => {
    setSelectedLimit(limit);
    refetch();
  };

  const formatPeriod = (entry: LeaderboardEntry) => {
    const start = new Date(entry.period_start).toLocaleDateString();
    const end = new Date(entry.period_end).toLocaleDateString();

    if (entry.period === "all_time") {
      return "All Time";
    }

    if (start === end) {
      return start;
    }

    return `${start} - ${end}`;
  };

  const getRankIcon = (rank: number) => {
    switch (rank) {
      case 1:
        return "🏆";
      case 2:
        return "🥈";
      case 3:
        return "🥉";
      default:
        return `#${rank}`;
    }
  };

  const getRankBadgeColor = (rank: number) => {
    switch (rank) {
      case 1:
        return "badge-warning";
      case 2:
        return "badge-neutral";
      case 3:
        return "badge-primary";
      default:
        return "badge-ghost";
    }
  };

  const getInputTypeIcon = (inputType: string) => {
    switch (inputType) {
      case "github":
        return "🐙";
      case "x":
        return "🐦";
      case "combined":
        return "🔗";
      default:
        return "📊";
    }
  };

  const getInputTypeLabel = (inputType: string) => {
    switch (inputType) {
      case "github":
        return "GitHub";
      case "x":
        return "X (Twitter)";
      case "combined":
        return "Combined";
      default:
        return inputType;
    }
  };

  const getDisplayName = (entry: LeaderboardEntry) => {
    // Priority: display_name > github_username > x_username > hash
    if (entry.display_name && entry.display_name.trim() !== "") {
      return entry.display_name;
    }
    if (entry.github_username) {
      return `@${entry.github_username}`;
    }
    if (entry.x_username) {
      return entry.x_username;
    }
    return `Developer #${entry.developer_hash.substring(0, 8)}`;
  };

  return (
    <div class={`leaderboard-container ${props.className || ""}`}>
      <div class="bg-base-100 rounded-lg shadow-lg p-6">
        {/* Header */}
        <div class="flex flex-col md:flex-row md:items-center md:justify-between mb-6">
          <div>
            <h2 class="text-3xl font-bold text-primary mb-2">🏆 Developer Leaderboard</h2>
            <p class="text-base-content/70">Top developers ranked by their "crackedness" score</p>
          </div>

          {/* Controls */}
          <div class="flex flex-col sm:flex-row gap-4 mt-4 md:mt-0">
            {/* Privacy Info Button */}
            <div class="dropdown dropdown-end">
              <div tabindex="0" role="button" class="btn btn-ghost btn-sm">
                🔒 Privacy
                <svg class="w-4 h-4 ml-2" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                  <path
                    stroke-linecap="round"
                    stroke-linejoin="round"
                    stroke-width="2"
                    d="M12 15v2m-6 4h12a2 2 0 002-2v-6a2 2 0 00-2-2H6a2 2 0 00-2 2v6a2 2 0 002 2zm10-10V7a4 4 0 00-8 0v4h8z"
                  ></path>
                </svg>
              </div>
              <div
                tabindex="0"
                class="dropdown-content z-[1] card card-compact w-80 p-2 shadow bg-base-100 text-base-content"
              >
                <div class="card-body">
                  <h3 class="card-title text-sm">🔒 Privacy & Data Protection</h3>
                  <div class="text-xs space-y-2">
                    <p>
                      <strong>Your Data:</strong> All analysis data is anonymized using SHA-256
                      hashing
                    </p>
                    <p>
                      <strong>Data Retention:</strong> Analysis data kept for 1 year, leaderboard
                      data for 90 days
                    </p>
                    <p>
                      <strong>Public Display:</strong> Only data with explicit consent appears on
                      leaderboards
                    </p>
                    <p>
                      <strong>Right to Delete:</strong> You can delete all your data at any time
                    </p>
                  </div>
                  <div class="card-actions justify-end">
                    <button
                      class="btn btn-primary btn-xs"
                      onClick={() => window.open("/privacy-policy", "_blank")}
                    >
                      View Privacy Policy
                    </button>
                  </div>
                </div>
              </div>
            </div>

            {/* Period Selector */}
            <div class="dropdown dropdown-end">
              <div tabindex="0" role="button" class="btn btn-outline">
                📅{" "}
                {PERIOD_OPTIONS.find((p) => p.value === selectedPeriod())?.label || "Select Period"}
                <svg class="w-4 h-4 ml-2" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                  <path
                    stroke-linecap="round"
                    stroke-linejoin="round"
                    stroke-width="2"
                    d="M19 9l-7 7-7-7"
                  ></path>
                </svg>
              </div>
              <ul
                tabindex="0"
                class="dropdown-content z-[1] menu p-2 shadow bg-base-100 rounded-box w-52"
              >
                <For each={PERIOD_OPTIONS}>
                  {(option) => (
                    <li>
                      <a
                        class={selectedPeriod() === option.value ? "active" : ""}
                        onClick={() => handlePeriodChange(option.value)}
                      >
                        <div class="flex flex-col">
                          <span class="font-semibold">{option.label}</span>
                          <span class="text-xs text-base-content/60">{option.description}</span>
                        </div>
                      </a>
                    </li>
                  )}
                </For>
              </ul>
            </div>

            {/* Limit Selector */}
            <div class="dropdown dropdown-end">
              <div tabindex="0" role="button" class="btn btn-outline">
                🎯 {LIMIT_OPTIONS.find((l) => l.value === selectedLimit())?.label || "Select Limit"}
                <svg class="w-4 h-4 ml-2" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                  <path
                    stroke-linecap="round"
                    stroke-linejoin="round"
                    stroke-width="2"
                    d="M19 9l-7 7-7-7"
                  ></path>
                </svg>
              </div>
              <ul
                tabindex="0"
                class="dropdown-content z-[1] menu p-2 shadow bg-base-100 rounded-box w-32"
              >
                <For each={LIMIT_OPTIONS}>
                  {(option) => (
                    <li>
                      <a
                        class={selectedLimit() === option.value ? "active" : ""}
                        onClick={() => handleLimitChange(option.value)}
                      >
                        {option.label}
                      </a>
                    </li>
                  )}
                </For>
              </ul>
            </div>
          </div>
        </div>

        {/* Loading State */}
        <Show when={isLoading()}>
          <div class="flex items-center justify-center py-12">
            <div class="loading loading-spinner loading-lg text-primary"></div>
            <span class="ml-3 text-lg">Loading leaderboard...</span>
          </div>
        </Show>

        {/* Error State */}
        <Show when={error()}>
          <div class="alert alert-error">
            <svg
              xmlns="http://www.w3.org/2000/svg"
              class="stroke-current shrink-0 h-6 w-6"
              fill="none"
              viewBox="0 0 24 24"
            >
              <path
                stroke-linecap="round"
                stroke-linejoin="round"
                stroke-width="2"
                d="M10 14l2-2m0 0l2-2m-2 2l-2-2m2 2l2 2m7-2a9 9 0 11-18 0 9 9 0 0118 0z"
              />
            </svg>
            <span>{error()}</span>
          </div>
        </Show>

        {/* Leaderboard Content */}
        <Show when={!isLoading() && !error() && leaderboardData()}>
          {(data) => (
            <div class="space-y-4">
              {/* Period Info */}
              <div class="bg-base-200 rounded-lg p-4">
                <div class="flex items-center justify-between">
                  <div>
                    <h3 class="text-lg font-semibold">
                      {PERIOD_OPTIONS.find((p) => p.value === data().period)?.label ||
                        data().period}{" "}
                      Leaderboard
                    </h3>
                    <p class="text-sm text-base-content/70">
                      Showing top {data().total} developers
                      <Show when={data().period !== "all_time"}>
                        <span> • {formatPeriod(data().entries[0])}</span>
                      </Show>
                    </p>
                  </div>
                  <div class="badge badge-primary badge-lg">{data().total} entries</div>
                </div>
              </div>

              {/* Leaderboard Entries */}
              <div class="space-y-2">
                <For each={data().entries}>
                  {(entry, index) => (
                    <div class="card bg-base-100 shadow-md hover:shadow-lg transition-shadow duration-200">
                      <div class="card-body p-4">
                        <div class="flex items-center justify-between">
                          {/* Rank and Avatar */}
                          <div class="flex items-center space-x-4">
                            <div
                              class={`badge badge-lg ${getRankBadgeColor(entry.rank)} font-bold text-lg px-3 py-1`}
                            >
                              {getRankIcon(entry.rank)}
                            </div>
                            <div class="avatar placeholder">
                              <div class="bg-neutral text-neutral-content rounded-full w-12">
                                <span class="text-lg">👨‍💻</span>
                              </div>
                            </div>
                            <div>
                              <div class="flex items-center space-x-2">
                                <h4 class="font-semibold text-lg">{getDisplayName(entry)}</h4>
                                <span
                                  class={`badge badge-sm ${entry.input_type === "combined" ? "badge-success" : entry.input_type === "github" ? "badge-info" : "badge-secondary"}`}
                                >
                                  {getInputTypeIcon(entry.input_type)}{" "}
                                  {getInputTypeLabel(entry.input_type)}
                                </span>
                              </div>
                              <p class="text-sm text-base-content/60">
                                Rank #{entry.rank} in {data().period}
                              </p>
                            </div>
                          </div>

                          {/* Score and Confidence */}
                          <div class="text-right">
                            <div class="text-2xl font-bold text-primary">
                              {entry.score.toFixed(1)}
                            </div>
                            <div class="text-sm text-base-content/70">
                              Confidence: {(entry.confidence * 100).toFixed(1)}%
                            </div>
                          </div>
                        </div>

                        {/* Progress Bar */}
                        <div class="mt-3">
                          <div class="flex justify-between text-sm mb-1">
                            <span>Score Progress</span>
                            <span>{entry.score.toFixed(1)} / 100</span>
                          </div>
                          <progress
                            class="progress progress-primary w-full"
                            value={entry.score}
                            max="100"
                          ></progress>
                        </div>
                      </div>
                    </div>
                  )}
                </For>
              </div>

              {/* Empty State */}
              <Show when={data().entries.length === 0}>
                <div class="text-center py-12">
                  <div class="text-6xl mb-4">🏆</div>
                  <h3 class="text-xl font-semibold mb-2">No entries yet</h3>
                  <p class="text-base-content/70">
                    Be the first developer to appear on this leaderboard! Run an analysis to get
                    started.
                  </p>
                </div>
              </Show>
            </div>
          )}
        </Show>
      </div>
    </div>
  );
}
