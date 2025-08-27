import { createSignal, onMount } from "solid-js";
import { getUserStats, UserStats as UserStatsType } from "../api";
import PaymentModal from "./PaymentModal";

interface UserStatsProps {
  onPaymentSuccess: () => void;
}

export default function UserStats(props: UserStatsProps) {
  const [userStats, setUserStats] = createSignal<UserStatsType | null>(null);
  const [loading, setLoading] = createSignal(true);
  const [error, setError] = createSignal<string | null>(null);
  const [showPaymentModal, setShowPaymentModal] = createSignal(false);

  const loadUserStats = async () => {
    try {
      setLoading(true);
      setError(null);
      const stats = await getUserStats();
      setUserStats(stats);
    } catch (err) {
      console.error("Failed to load user stats:", err);
      setError(err instanceof Error ? err.message : "Failed to load user stats");
    } finally {
      setLoading(false);
    }
  };

  onMount(() => {
    loadUserStats();
  });

  const getUsagePercentage = () => {
    const stats = userStats();
    if (!stats || stats.is_paid) return 0;
    return (stats.requests_this_week / 5) * 100;
  };

  const getUsageColor = () => {
    const percentage = getUsagePercentage();
    if (percentage >= 100) return "error";
    if (percentage >= 80) return "warning";
    return "success";
  };

  const formatDate = (dateString: string) => {
    return new Date(dateString).toLocaleDateString();
  };

  if (loading()) {
    return (
      <div class="card bg-base-200 shadow-xl">
        <div class="card-body">
          <div class="flex items-center gap-3">
            <span class="loading loading-spinner loading-sm"></span>
            <span>Loading usage stats...</span>
          </div>
        </div>
      </div>
    );
  }

  if (error()) {
    return (
      <div class="card bg-base-200 shadow-xl">
        <div class="card-body">
          <div class="alert alert-error">
            <span>⚠️ {error()}</span>
          </div>
          <button onClick={loadUserStats} class="btn btn-sm btn-outline">
            Retry
          </button>
        </div>
      </div>
    );
  }

  const stats = userStats();
  if (!stats) return null;

  return (
    <>
      <div class="card bg-base-200 shadow-xl">
        <div class="card-body">
          <div class="flex items-center justify-between mb-4">
            <h3 class="card-title text-lg">Usage Stats</h3>
            {stats.is_paid && <div class="badge badge-success">Premium</div>}
          </div>

          <div class="space-y-4">
            {/* Usage Progress */}
            <div>
              <div class="flex justify-between text-sm mb-2">
                <span>Weekly Requests</span>
                <span
                  class={`font-semibold ${stats.requests_this_week >= 5 && !stats.is_paid ? "text-error" : ""}`}
                >
                  {stats.requests_this_week}/5
                </span>
              </div>
              {!stats.is_paid && (
                <progress
                  class={`progress progress-${getUsageColor()}`}
                  value={stats.requests_this_week}
                  max="5"
                />
              )}
              {stats.is_paid && <div class="text-success font-semibold">🎉 Unlimited Access</div>}
            </div>

            {/* Remaining Requests */}
            <div class="flex justify-between items-center">
              <span class="text-sm">Remaining this week:</span>
              <span
                class={`font-bold text-lg ${stats.remaining_requests <= 1 && !stats.is_paid ? "text-error" : "text-success"}`}
              >
                {stats.remaining_requests === -1 ? "∞" : stats.remaining_requests}
              </span>
            </div>

            {/* Week Info */}
            <div class="text-xs text-base-content/70">
              Week: {formatDate(stats.week_start)} - {formatDate(stats.week_end)}
            </div>

            {/* Action Buttons */}
            <div class="flex gap-2 pt-2">
              {!stats.is_paid && stats.requests_this_week >= 5 && (
                <button
                  onClick={() => setShowPaymentModal(true)}
                  class="btn btn-primary btn-sm flex-1"
                >
                  🚀 Upgrade Now
                </button>
              )}
              {!stats.is_paid && stats.requests_this_week < 5 && (
                <button onClick={() => setShowPaymentModal(true)} class="btn btn-outline btn-sm">
                  💝 Donate
                </button>
              )}
              {stats.is_paid && (
                <button onClick={() => setShowPaymentModal(true)} class="btn btn-outline btn-sm">
                  💝 Donate
                </button>
              )}
            </div>
          </div>
        </div>
      </div>

      <PaymentModal
        isOpen={showPaymentModal()}
        onClose={() => setShowPaymentModal(false)}
        userStats={stats}
        onPaymentSuccess={() => {
          setShowPaymentModal(false);
          loadUserStats(); // Refresh stats
          props.onPaymentSuccess();
        }}
      />
    </>
  );
}
