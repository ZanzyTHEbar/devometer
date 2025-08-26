import { createSignal, createEffect } from "solid-js";

type MeterProps = {
  value?: number; // 0-100
};

export default function Meter(props: MeterProps) {
  const [angle, setAngle] = createSignal(0);

  createEffect(() => {
    const v = Math.max(0, Math.min(100, props.value ?? 50));
    // map 0..100 to -120deg .. 120deg
    const a = (v / 100) * 240 - 120;
    setAngle(a);
  });

  return (
    <svg
      viewBox="0 0 200 120"
      width="100%"
      height="100%"
      class="pointer-events-none"
    >
      <defs>
        <linearGradient id="g1" x1="0" x2="1">
          <stop offset="0%" stop-color="#06b6d4" />
          <stop offset="50%" stop-color="#7c3aed" />
          <stop offset="100%" stop-color="#ef4444" />
        </linearGradient>
      </defs>

      <g transform="translate(100,100)">
        <path
          d="M-80,0 A80,80 0 0,1 80,0"
          fill="none"
          stroke="#e6e6e6"
          stroke-width="12"
          stroke-linecap="round"
        />
        <path
          d="M-78,0 A78,78 0 0,1 78,0"
          fill="none"
          stroke="url(#g1)"
          stroke-width="10"
          stroke-linecap="round"
          stroke-dasharray="245"
          stroke-dashoffset="0"
          opacity="0.9"
        />

        {/* needle */}
        <g transform={`rotate(${angle()} )`}>
          <line
            x1="0"
            y1="0"
            x2="0"
            y2="-64"
            stroke="#111827"
            stroke-width="4"
            stroke-linecap="round"
          />
          <circle cx="0" cy="0" r="6" fill="#111827" />
        </g>

        {/* ticks */}
        {Array.from({ length: 9 }).map((_, i) => {
          const t = -120 + i * 30;
          return (
            <g transform={`rotate(${t}) translate(0,-74)`}>
              <rect x="-1" y="0" width="2" height="8" fill="#374151" />
            </g>
          );
        })}

        <text x="0" y="48" text-anchor="middle" font-size="12" fill="#374151">
          Crackedness
        </text>
      </g>
    </svg>
  );
}
