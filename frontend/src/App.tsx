import { createSignal } from "solid-js";
import Meter from "./components/Meter";

export default function App() {
  const [input, setInput] = createSignal("");
  const [loading, setLoading] = createSignal(false);
  const [result, setResult] = createSignal<any>(null);

  return (
    <div class="min-h-screen flex items-center justify-center bg-slate-50">
      <div class="w-[760px] p-8 rounded-xl shadow-lg glass">
        <h1 class="text-2xl font-semibold mb-4">Cracked Dev-o-Meter</h1>
        <p class="text-sm text-slate-600 mb-6">
          Enter an X account or GitHub repo to evaluate crackedness.
        </p>

        <div class="flex gap-4">
          <input
            class="flex-1 p-3 rounded-md border border-slate-200"
            placeholder="e.g. github.com/user/repo or x.com/username"
            value={input()}
            onInput={(e) => setInput((e.target as HTMLInputElement).value)}
          />
          <button
            class="px-4 py-3 bg-indigo-600 text-white rounded-md"
            onClick={async () => {
              setLoading(true);
              setResult(null);
              try {
                const data = await (
                  await fetch("/analyze", {
                    method: "POST",
                    headers: { "Content-Type": "application/json" },
                    body: JSON.stringify({ input: input() }),
                  })
                ).json();
                setResult(data);
              } catch (e) {
                setResult({ error: (e as Error).message });
              } finally {
                setLoading(false);
              }
            }}
            disabled={loading()}
          >
            {loading() ? "Analyzing…" : "Analyze"}
          </button>
        </div>

        <div class="mt-8">
          <div class="h-48 bg-white rounded-lg border border-slate-100 flex items-center justify-center p-4">
            <div style={{ width: "320px", height: "160px" }}>
              <Meter value={50} />
            </div>
          </div>
        </div>
      </div>
    </div>
  );
}
