import { createSignal, Show } from "solid-js";

interface LeaderboardOptInModalProps {
  isOpen: boolean;
  onClose: () => void;
  onOptIn: (optIn: boolean, displayName: string) => Promise<void>;
  developerHash: string;
  score: number;
  githubUsername?: string;
  xUsername?: string;
}

export default function LeaderboardOptInModal(props: LeaderboardOptInModalProps) {
  const [displayName, setDisplayName] = createSignal("");
  const [isSubmitting, setIsSubmitting] = createSignal(false);
  const [useRealName, setUseRealName] = createSignal(true);

  const handleSubmit = async (optIn: boolean) => {
    setIsSubmitting(true);
    try {
      await props.onOptIn(optIn, useRealName() ? displayName() : "");
      props.onClose();
    } catch (error) {
      console.error("Failed to submit opt-in:", error);
    } finally {
      setIsSubmitting(false);
    }
  };

  return (
    <Show when={props.isOpen}>
      <div class="modal modal-open">
        <div class="modal-box max-w-2xl">
          <h3 class="font-bold text-2xl mb-4">🏆 Add to Leaderboard?</h3>

          <div class="bg-primary/10 rounded-lg p-4 mb-6">
            <div class="flex items-center justify-between">
              <div>
                <div class="text-sm text-base-content/70">Your Crackedness Score</div>
                <div class="text-4xl font-bold text-primary">{props.score.toFixed(1)}</div>
              </div>
              <div class="text-6xl">🎯</div>
            </div>
          </div>

          <p class="mb-4">
            Want to compete with other developers? Add your score to the public leaderboard!
          </p>

          <div class="form-control mb-6">
            <label class="label cursor-pointer justify-start gap-3">
              <input
                type="checkbox"
                class="checkbox"
                checked={useRealName()}
                onChange={(e) => setUseRealName(e.currentTarget.checked)}
              />
              <span class="label-text">
                Show my {props.githubUsername || props.xUsername ? "username" : "display name"}
              </span>
            </label>

            <Show when={useRealName()}>
              <div class="mt-3">
                <label class="label">
                  <span class="label-text">Display Name (optional)</span>
                </label>
                <input
                  type="text"
                  placeholder="Enter a display name or leave blank to use username"
                  class="input input-bordered w-full"
                  value={displayName()}
                  onInput={(e) => setDisplayName(e.currentTarget.value)}
                  maxLength={50}
                />
                <label class="label">
                  <span class="label-text-alt text-base-content/60">
                    Leave blank to show your GitHub/X username
                  </span>
                </label>
              </div>
            </Show>

            <Show when={!useRealName()}>
              <div class="alert alert-info mt-3">
                <svg
                  xmlns="http://www.w3.org/2000/svg"
                  fill="none"
                  viewBox="0 0 24 24"
                  class="stroke-current shrink-0 w-6 h-6"
                >
                  <path
                    stroke-linecap="round"
                    stroke-linejoin="round"
                    stroke-width="2"
                    d="M13 16h-1v-4h-1m1-4h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z"
                  />
                </svg>
                <span>You'll appear as an anonymized developer ID</span>
              </div>
            </Show>
          </div>

          <div class="bg-base-200 rounded-lg p-4 mb-6">
            <h4 class="font-semibold mb-2">🔒 Privacy Information</h4>
            <ul class="text-sm space-y-1 text-base-content/70">
              <li>• Your analysis data is anonymized using SHA-256 hashing</li>
              <li>• Only scores with your consent appear on public leaderboards</li>
              <li>• You can remove your data at any time</li>
              <li>• Leaderboard data is retained for 90 days</li>
            </ul>
          </div>

          <div class="modal-action">
            <button
              class="btn btn-ghost"
              onClick={() => handleSubmit(false)}
              disabled={isSubmitting()}
            >
              No Thanks
            </button>
            <button
              class="btn btn-primary"
              onClick={() => handleSubmit(true)}
              disabled={isSubmitting()}
            >
              <Show when={isSubmitting()} fallback="Add to Leaderboard 🚀">
                <span class="loading loading-spinner" />
                Saving...
              </Show>
            </button>
          </div>
        </div>
      </div>
    </Show>
  );
}
