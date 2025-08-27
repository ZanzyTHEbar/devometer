import { createSignal, createEffect, For, type Component } from "solid-js";

interface MeterProps {
  value?: number; // 0-100
  animated?: boolean;
}

const Meter: Component<MeterProps> = (props) => {
  const [angle, setAngle] = createSignal(0);
  const [isAnimating, setIsAnimating] = createSignal(false);

  // Add subtle pulse animation for the center hub when animating
  const hubRadius = () => (isAnimating() ? 9 : 8);

  createEffect(() => {
    const v = Math.max(0, Math.min(100, props.value ?? 50));
    const targetAngle = (v / 100) * 240 - 120; // map 0..100 to -120deg .. 120deg

    if (props.animated !== false) {
      setIsAnimating(true);
      // Smooth animation
      const startAngle = angle();
      const duration = 1000; // 1 second
      const startTime = Date.now();

      const animate = () => {
        const elapsed = Date.now() - startTime;
        const progress = Math.min(elapsed / duration, 1);

        // Easing function for smooth animation
        const easeOutCubic = 1 - Math.pow(1 - progress, 3);
        const currentAngle = startAngle + (targetAngle - startAngle) * easeOutCubic;

        setAngle(currentAngle);

        if (progress < 1) {
          requestAnimationFrame(animate);
        } else {
          setIsAnimating(false);
        }
      };

      requestAnimationFrame(animate);
    } else {
      setAngle(targetAngle);
    }
  });

  // Generate tick marks
  const ticks = () => {
    return Array.from({ length: 9 }, (_, i) => {
      const tickAngle = -120 + i * 30;
      const isMainTick = i % 2 === 0;

      // Adjust label positioning for extreme angles
      let labelOffset = { x: 0, y: -16 };
      let textAnchor = "middle";

      if (tickAngle === 0) {
        // Top "50" label: push up for more clearance from arc
        labelOffset = { x: 0, y: -20 };
        textAnchor = "middle";
      } else if (Math.abs(tickAngle) === 120) {
        // Extreme angles: adjust positioning to prevent cutoff
        const direction = tickAngle > 0 ? 1 : -1;
        labelOffset = { x: direction * 5, y: -13 };
        textAnchor = tickAngle > 0 ? "start" : "end";
      } else if (Math.abs(tickAngle) === 60) {
        // Moderate angles: slight adjustment for better readability
        const direction = tickAngle > 0 ? 1 : -1;
        labelOffset = { x: direction * 2, y: -15 };
        textAnchor = "middle";
      }

      return {
        angle: tickAngle,
        isMain: isMainTick,
        label: isMainTick ? `${(i * 12.5).toFixed(0)}` : null,
        labelOffset,
        textAnchor,
      };
    });
  };

  return (
    <svg
      viewBox="-24 -20 248 160"
      width="100%"
      height="100%"
      class="pointer-events-none"
      role="img"
      aria-label={`Crackedness meter showing ${props.value || 50}%`}
    >
      <defs>
        <linearGradient id="meterGradient" x1="0" x2="1">
          <stop offset="0%" stop-color="#06b6d4" />
          <stop offset="50%" stop-color="#7c3aed" />
          <stop offset="100%" stop-color="#ef4444" />
        </linearGradient>
        <radialGradient id="hubGradient" cx="50%" cy="50%" r="50%">
          <stop offset="0%" stop-color="#f8fafc" />
          <stop offset="70%" stop-color="#e6e6e6" />
          <stop offset="100%" stop-color="#d1d5db" />
        </radialGradient>
        <filter id="glow">
          <feGaussianBlur stdDeviation="3" result="coloredBlur" />
          <feMerge>
            <feMergeNode in="coloredBlur" />
            <feMergeNode in="SourceGraphic" />
          </feMerge>
        </filter>
        <filter id="shadow">
          <feDropShadow dx="2" dy="2" stdDeviation="2" flood-color="#00000020" />
        </filter>
      </defs>

      <g transform="translate(100,100)" filter="url(#shadow)">
        {/* Background arc */}
        <path
          d="M-80,0 A80,80 0 0,1 80,0"
          fill="none"
          stroke="#e6e6e6"
          stroke-width="12"
          stroke-linecap="round"
          opacity="0.3"
        />

        {/* Active arc with gradient */}
        <path
          d="M-78,0 A78,78 0 0,1 78,0"
          fill="none"
          stroke="url(#meterGradient)"
          stroke-width="10"
          stroke-linecap="round"
          stroke-dasharray="245"
          stroke-dashoffset="0"
          opacity="0.9"
          filter={isAnimating() ? "url(#glow)" : undefined}
        />

        {/* Needle with glow effect */}
        <g transform={`rotate(${angle()})`}>
          <line
            x1="0"
            y1="0"
            x2="0"
            y2="-64"
            stroke="#111827"
            stroke-width="4"
            stroke-linecap="round"
            filter={isAnimating() ? "url(#glow)" : undefined}
          />
          <circle
            cx="0"
            cy="0"
            r="6"
            fill="#111827"
            filter={isAnimating() ? "url(#glow)" : undefined}
          />
        </g>

        {/* Tick marks - secondary ticks made shorter and lighter */}
        <For each={ticks()}>
          {(tick) => (
            <g transform={`rotate(${tick.angle}) translate(0,-74)`}>
              <rect
                x={tick.isMain ? "-1.5" : "-1"}
                y="0"
                width={tick.isMain ? "3" : "2"}
                height={tick.isMain ? "12" : "6"}
                fill="#374151"
                opacity={tick.isMain ? "0.7" : "0.4"}
              />
              {tick.label && (
                <text
                  x={tick.labelOffset.x}
                  y={tick.labelOffset.y}
                  text-anchor={tick.textAnchor}
                  font-size="7"
                  fill="#6b7280"
                  transform={`rotate(${-tick.angle})`}
                >
                  {tick.label}
                </text>
              )}
            </g>
          )}
        </For>

        {/* Center hub with subtle pulse animation */}
        <circle
          cx="0"
          cy="0"
          r={hubRadius()}
          fill="url(#hubGradient)"
          stroke="#374151"
          stroke-width="2"
        />

        {/* Label */}
        <text x="0" y="48" text-anchor="middle" font-size="12" fill="#374151" font-weight="500">
          Crackedness
        </text>

        {/* Score display - moved down to clear bottom ticks */}
        <text x="0" y="36" text-anchor="middle" font-size="16" fill="#1f2937" font-weight="bold">
          {Math.round(props.value ?? 50)}%
        </text>
      </g>
    </svg>
  );
};

export default Meter;
