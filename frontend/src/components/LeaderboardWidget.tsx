import { createSignal, createResource, For, Show } from "solid-js";
import { LeaderboardEntry } from "../api";

interface LeaderboardWidgetProps {
  className?: string;
}

export default function LeaderboardWidget(props: LeaderboardWidgetProps) {
  const [selectedPeriod, setSelectedPeriod] = createSignal("weekly");

  const fetchTop10 = async (period: string): Promise<LeaderboardEntry[]> => {
    const response = await fetch(`/api/leaderboard/${period}?limit=10`);
    if (!response.ok) throw new Error("Failed to fetch top 10");
    const data = await response.json();
    return data.entries;
  };

  const [top10Data] = createResource(selectedPeriod, fetchTop10);

  const getDisplayName = (entry: LeaderboardEntry) => {
    if (entry.display_name) return entry.display_name;
    if (entry.github_username) return `@${entry.github_username}`;
    if (entry.x_username) return entry.x_username;
    return `Dev #${entry.developer_hash.substring(0, 6)}`;
  };

  const getRankMedal = (rank: number) => {
    if (rank === 1) return "🥇";
    if (rank === 2) return "🥈";
    if (rank === 3) return "🥉";
    return rank;
  };

  return (
    <div
      class={`card bg-gradient-to-br from-primary/5 to-secondary/5 shadow-xl ${props.className || ""}`}
    >
      <div class="card-body">
        <div class="flex items-center justify-between mb-4">
          <h3 class="card-title text-lg">🏆 Top 10 Developers</h3>
          <select
            class="select select-sm select-bordered"
            value={selectedPeriod()}
            onChange={(e) => setSelectedPeriod(e.currentTarget.value)}
          >
            <option value="daily">Today</option>
            <option value="weekly">This Week</option>
            <option value="monthly">This Month</option>
            <option value="all_time">All Time</option>
          </select>
        </div>

        <Show when={top10Data.loading}>
          <div class="flex justify-center py-4">
            <span class="loading loading-spinner loading-md" />
          </div>
        </Show>

        <Show when={!top10Data.loading && top10Data()}>
          <div class="space-y-2">
            <For each={top10Data()}>
              {(entry) => (
                <div class="flex items-center justify-between p-2 rounded-lg hover:bg-base-200 transition-colors">
                  <div class="flex items-center gap-3">
                    <div class="font-bold text-lg w-8 text-center">{getRankMedal(entry.rank)}</div>
                    <div>
                      <div class="font-medium text-sm">{getDisplayName(entry)}</div>
                      <div class="text-xs text-base-content/60">
                        {entry.input_type === "combined"
                          ? "🔗"
                          : entry.input_type === "github"
                            ? "🐙"
                            : "🐦"}
                      </div>
                    </div>
                  </div>
                  <div class="text-right">
                    <div class="font-bold text-primary">{entry.score.toFixed(1)}</div>
                    <div class="text-xs text-base-content/60">
                      {(entry.confidence * 100).toFixed(0)}%
                    </div>
                  </div>
                </div>
              )}
            </For>
          </div>
        </Show>

        <div class="card-actions justify-end mt-4">
          <button
            class="btn btn-sm btn-ghost"
            onClick={() => {
              // Navigate to full leaderboard by changing active tab
              const event = new CustomEvent("navigate-leaderboard");
              window.dispatchEvent(event);
            }}
          >
            View Full Leaderboard →
          </button>
        </div>
      </div>
    </div>
  );
}
