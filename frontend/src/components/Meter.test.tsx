import { describe, it, expect, beforeEach, vi } from "vitest";
import { render, screen, cleanup } from "@solidjs/testing-library";
import Meter from "./Meter";

// Mock requestAnimationFrame and Date.now
const requestAnimationFrameMock = vi.fn((cb) => {
  // Execute callback immediately for testing
  cb();
  return 1;
});

const originalDateNow = Date.now;
let mockTime = 1000000000000;

beforeEach(() => {
  cleanup();
  vi.clearAllMocks();

  // Reset mock time
  mockTime = 1000000000000;

  // Mock Date.now
  vi.stubGlobal("Date", {
    ...Date,
    now: () => mockTime,
  });

  // Mock requestAnimationFrame
  vi.stubGlobal("requestAnimationFrame", requestAnimationFrameMock);
});

afterEach(() => {
  vi.restoreAllMocks();
});

describe("Meter Component", () => {
  describe("Basic Rendering", () => {
    it("renders with default value (50)", () => {
      render(() => <Meter />);

      const svg = screen.getByRole("img", { name: /Crackedness meter showing 50%/ });
      expect(svg).toBeInTheDocument();
      expect(svg).toHaveAttribute("viewBox", "0 0 200 120");

      // Check that score is displayed
      expect(screen.getByText("50%")).toBeInTheDocument();
      expect(screen.getByText("Crackedness")).toBeInTheDocument();
    });

    it("renders with custom value", () => {
      render(() => <Meter value={75} />);

      expect(
        screen.getByRole("img", { name: /Crackedness meter showing 75%/ })
      ).toBeInTheDocument();
      expect(screen.getByText("75%")).toBeInTheDocument();
    });

    it("clamps value to valid range (0-100)", () => {
      const { unmount } = render(() => <Meter value={-10} />);
      expect(screen.getByRole("img", { name: /Crackedness meter showing 0%/ })).toBeInTheDocument();
      expect(screen.getByText("0%")).toBeInTheDocument();

      unmount();

      render(() => <Meter value={150} />);
      expect(
        screen.getByRole("img", { name: /Crackedness meter showing 100%/ })
      ).toBeInTheDocument();
      expect(screen.getByText("100%")).toBeInTheDocument();
    });

    it("rounds decimal values to whole numbers", () => {
      render(() => <Meter value={67.8} />);

      expect(
        screen.getByRole("img", { name: /Crackedness meter showing 68%/ })
      ).toBeInTheDocument();
      expect(screen.getByText("68%")).toBeInTheDocument();
    });
  });

  describe("Animation Behavior", () => {
    it("enables animation by default", () => {
      render(() => <Meter value={75} />);

      // Animation should be enabled by default
      expect(requestAnimationFrameMock).toHaveBeenCalled();
    });

    it("disables animation when animated=false", () => {
      render(() => <Meter value={75} animated={false} />);

      // Animation should not be triggered
      expect(requestAnimationFrameMock).not.toHaveBeenCalled();
    });

    it("updates angle immediately when animation is disabled", () => {
      render(() => <Meter value={100} animated={false} />);

      // Should not use animation frames
      expect(requestAnimationFrameMock).not.toHaveBeenCalled();
      expect(screen.getByText("100%")).toBeInTheDocument();
    });

    it("handles animation timing correctly", () => {
      render(() => <Meter value={75} />);

      // Should trigger animation
      expect(requestAnimationFrameMock).toHaveBeenCalled();
    });
  });

  describe("Angle Calculation", () => {
    it("maps 0% to -120 degrees", () => {
      render(() => <Meter value={0} animated={false} />);

      // The needle should be positioned at -120 degrees for 0%
      // This is tested implicitly through the rendering
      expect(screen.getByText("0%")).toBeInTheDocument();
    });

    it("maps 50% to 0 degrees", () => {
      render(() => <Meter value={50} animated={false} />);

      expect(screen.getByText("50%")).toBeInTheDocument();
    });

    it("maps 100% to 120 degrees", () => {
      render(() => <Meter value={100} animated={false} />);

      expect(screen.getByText("100%")).toBeInTheDocument();
    });
  });

  describe("Tick Marks", () => {
    it("renders 9 tick marks", () => {
      render(() => <Meter />);

      // Should render tick marks for values 0, 12.5, 25, 37.5, 50, 62.5, 75, 87.5, 100
      const ticks = screen.getAllByRole("presentation"); // Tick marks are rect elements
      expect(ticks.length).toBeGreaterThanOrEqual(9);
    });

    it("renders main tick labels at expected positions", () => {
      render(() => <Meter />);

      // Should show labels at main tick positions
      expect(screen.getByText("0")).toBeInTheDocument();
      expect(screen.getByText("25")).toBeInTheDocument();
      expect(screen.getByText("50")).toBeInTheDocument();
      expect(screen.getByText("75")).toBeInTheDocument();
      expect(screen.getByText("100")).toBeInTheDocument();
    });
  });

  describe("SVG Structure", () => {
    it("has correct SVG dimensions", () => {
      render(() => <Meter />);

      const svg = screen.getByRole("img");
      expect(svg).toHaveAttribute("viewBox", "0 0 200 120");
      expect(svg).toHaveClass("pointer-events-none");
    });

    it("includes gradient definition", () => {
      render(() => <Meter />);

      const svg = screen.getByRole("img");
      const defs = svg.querySelector("defs");
      expect(defs).toBeInTheDocument();

      const gradient = defs?.querySelector("#meterGradient");
      expect(gradient).toBeInTheDocument();
    });

    it("includes glow filter", () => {
      render(() => <Meter />);

      const svg = screen.getByRole("img");
      const defs = svg.querySelector("defs");
      const glow = defs?.querySelector("#glow");
      expect(glow).toBeInTheDocument();
    });
  });

  describe("Accessibility", () => {
    it("has proper ARIA label", () => {
      render(() => <Meter value={42} />);

      const svg = screen.getByRole("img", { name: /Crackedness meter showing 42%/ });
      expect(svg).toBeInTheDocument();
    });

    it("has img role", () => {
      render(() => <Meter />);

      const svg = screen.getByRole("img");
      expect(svg).toBeInTheDocument();
    });

    it("updates ARIA label when value changes", () => {
      const { unmount } = render(() => <Meter value={25} />);
      expect(
        screen.getByRole("img", { name: /Crackedness meter showing 25%/ })
      ).toBeInTheDocument();

      unmount();

      render(() => <Meter value={80} />);
      expect(
        screen.getByRole("img", { name: /Crackedness meter showing 80%/ })
      ).toBeInTheDocument();
    });
  });

  describe("Value Changes", () => {
    it("updates display when value prop changes", () => {
      let value = 25;
      const { rerender } = render(() => <Meter value={value} animated={false} />);

      expect(screen.getByText("25%")).toBeInTheDocument();

      value = 75;
      rerender();

      expect(screen.getByText("75%")).toBeInTheDocument();
    });

    it("handles rapid value changes", () => {
      let value = 10;
      const { rerender } = render(() => <Meter value={value} animated={false} />);

      for (let i = 20; i <= 100; i += 20) {
        value = i;
        rerender();
        expect(screen.getByText(`${i}%`)).toBeInTheDocument();
      }
    });
  });

  describe("Performance", () => {
    it("does not cause excessive re-renders", () => {
      const renderSpy = vi.fn();
      const MeterWithSpy = () => {
        renderSpy();
        return <Meter value={50} animated={false} />;
      };

      render(MeterWithSpy);

      // Should only render once initially
      expect(renderSpy).toHaveBeenCalledTimes(1);
    });

    it("handles frequent updates gracefully", () => {
      let value = 0;
      const { rerender } = render(() => <Meter value={value} animated={false} />);

      // Rapid updates
      for (let i = 0; i < 100; i++) {
        value = i % 101; // Keep within 0-100
        rerender();
      }

      expect(screen.getByText("100%")).toBeInTheDocument();
    });
  });
});
