import { describe, it, expect, beforeEach, vi } from "vitest";
import { render, screen, fireEvent, waitFor, cleanup } from "@solidjs/testing-library";
import App from "./App";
import { analyze } from "./api";

// Mock the API
vi.mock("./api", () => ({
  analyze: vi.fn(),
}));

const mockAnalyze = vi.mocked(analyze);

beforeEach(() => {
  cleanup();
  vi.clearAllMocks();
});

describe("App Component", () => {
  describe("Initial Rendering", () => {
    it("renders the main application layout", () => {
      render(() => <App />);

      expect(screen.getByText("Cracked Dev-o-Meter")).toBeInTheDocument();
      expect(screen.getByText(/Enter a GitHub repo or username/)).toBeInTheDocument();
      expect(
        screen.getByPlaceholderText(/e.g., github.com\/user\/repo or @username/)
      ).toBeInTheDocument();
      expect(screen.getByRole("button", { name: "Analyze" })).toBeInTheDocument();
    });

    it("renders the meter with default value", () => {
      render(() => <App />);

      const meter = screen.getByRole("img", { name: /Crackedness meter showing 50%/ });
      expect(meter).toBeInTheDocument();
      expect(screen.getByText("50%")).toBeInTheDocument();
    });

    it("shows initial instruction message", () => {
      render(() => <App />);

      expect(
        screen.getByText(/Enter a GitHub repo or username above to see the crackedness analysis/)
      ).toBeInTheDocument();
    });
  });

  describe("Input Handling", () => {
    it("updates input value when typing", () => {
      render(() => <App />);

      const input = screen.getByPlaceholderText(/e.g., github.com\/user\/repo or @username/);
      fireEvent.input(input, { target: { value: "facebook/react" } });

      expect(input).toHaveValue("facebook/react");
    });

    it("enables analyze button when input has content", () => {
      render(() => <App />);

      const input = screen.getByPlaceholderText(/e.g., github.com\/user\/repo or @username/);
      const button = screen.getByRole("button", { name: "Analyze" });

      // Initially disabled
      expect(button).toBeDisabled();

      // Enable when input has content
      fireEvent.input(input, { target: { value: "facebook/react" } });
      expect(button).not.toBeDisabled();
    });

    it("disables analyze button when input is empty or whitespace", () => {
      render(() => <App />);

      const input = screen.getByPlaceholderText(/e.g., github.com\/user\/repo or @username/);
      const button = screen.getByRole("button", { name: "Analyze" });

      // Test empty input
      fireEvent.input(input, { target: { value: "" } });
      expect(button).toBeDisabled();

      // Test whitespace only
      fireEvent.input(input, { target: { value: "   " } });
      expect(button).toBeDisabled();

      // Test valid input
      fireEvent.input(input, { target: { value: "facebook/react" } });
      expect(button).not.toBeDisabled();
    });
  });

  describe("Form Submission", () => {
    it("handles Enter key press", () => {
      render(() => <App />);

      const input = screen.getByPlaceholderText(/e.g., github.com\/user\/repo or @username/);
      fireEvent.input(input, { target: { value: "facebook/react" } });
      fireEvent.keyDown(input, { key: "Enter" });

      expect(mockAnalyze).toHaveBeenCalledWith("facebook/react");
    });

    it("handles button click", () => {
      render(() => <App />);

      const input = screen.getByPlaceholderText(/e.g., github.com\/user\/repo or @username/);
      const button = screen.getByRole("button", { name: "Analyze" });

      fireEvent.input(input, { target: { value: "octocat" } });
      fireEvent.click(button);

      expect(mockAnalyze).toHaveBeenCalledWith("octocat");
    });

    it("does not submit when input is empty", () => {
      render(() => <App />);

      const input = screen.getByPlaceholderText(/e.g., github.com\/user\/repo or @username/);
      const button = screen.getByRole("button", { name: "Analyze" });

      fireEvent.input(input, { target: { value: "" } });
      fireEvent.click(button);

      expect(mockAnalyze).not.toHaveBeenCalled();
    });

    it("does not submit when input is whitespace only", () => {
      render(() => <App />);

      const input = screen.getByPlaceholderText(/e.g., github.com\/user\/repo or @username/);
      const button = screen.getByRole("button", { name: "Analyze" });

      fireEvent.input(input, { target: { value: "   " } });
      fireEvent.click(button);

      expect(mockAnalyze).not.toHaveBeenCalled();
    });
  });

  describe("Analysis Flow", () => {
    it("shows loading state during analysis", async () => {
      // Mock a delayed response
      mockAnalyze.mockImplementation(() => new Promise((resolve) => setTimeout(resolve, 100)));

      render(() => <App />);

      const input = screen.getByPlaceholderText(/e.g., github.com\/user\/repo or @username/);
      const button = screen.getByRole("button", { name: "Analyze" });

      fireEvent.input(input, { target: { value: "facebook/react" } });
      fireEvent.click(button);

      // Check loading state
      expect(screen.getByText("Analyzing...")).toBeInTheDocument();
      expect(button).toBeDisabled();
      expect(input).toBeDisabled();
    });

    it("displays analysis results successfully", async () => {
      const mockResult = {
        score: 85,
        confidence: 0.9,
        posterior: 0.85,
        contributors: [
          { name: "influence.stars", contribution: 2.5 },
          { name: "influence.forks", contribution: 1.8 },
          { name: "influence.followers", contribution: 2.2 },
        ],
        breakdown: {
          shipping: 0.8,
          quality: 0.7,
          influence: 0.85,
          complexity: 0.6,
          collaboration: 0.5,
          reliability: 0.9,
          novelty: 0.4,
        },
      };

      mockAnalyze.mockResolvedValue(mockResult);

      render(() => <App />);

      const input = screen.getByPlaceholderText(/e.g., github.com\/user\/repo or @username/);
      const button = screen.getByRole("button", { name: "Analyze" });

      fireEvent.input(input, { target: { value: "facebook/react" } });
      fireEvent.click(button);

      await waitFor(() => {
        expect(screen.getByText("85/100")).toBeInTheDocument();
        expect(screen.getByText("Crackedness Score")).toBeInTheDocument();
        expect(screen.getByText("Category Breakdown")).toBeInTheDocument();
      });

      // Check score display
      expect(screen.getByText("85/100")).toBeInTheDocument();

      // Check confidence display
      expect(screen.getByText(/Confidence: 90.0%/)).toBeInTheDocument();

      // Check category breakdown
      expect(screen.getByText("Shipping:")).toBeInTheDocument();
      expect(screen.getByText("80%")).toBeInTheDocument();
      expect(screen.getByText("Quality:")).toBeInTheDocument();
      expect(screen.getByText("70%")).toBeInTheDocument();
      expect(screen.getByText("Influence:")).toBeInTheDocument();
      expect(screen.getByText("85%")).toBeInTheDocument();
    });

    it("updates meter value when analysis completes", async () => {
      const mockResult = {
        score: 75,
        confidence: 0.8,
        posterior: 0.75,
        contributors: [],
        breakdown: {
          shipping: 0.6,
          quality: 0.7,
          influence: 0.75,
          complexity: 0.5,
          collaboration: 0.4,
          reliability: 0.8,
          novelty: 0.3,
        },
      };

      mockAnalyze.mockResolvedValue(mockResult);

      render(() => <App />);

      const input = screen.getByPlaceholderText(/e.g., github.com\/user\/repo or @username/);
      const button = screen.getByRole("button", { name: "Analyze" });

      fireEvent.input(input, { target: { value: "facebook/react" } });
      fireEvent.click(button);

      await waitFor(() => {
        const meter = screen.getByRole("img", { name: /Crackedness meter showing 75%/ });
        expect(meter).toBeInTheDocument();
        expect(screen.getByText("75%")).toBeInTheDocument();
      });
    });

    it("displays top contributors when available", async () => {
      const mockResult = {
        score: 90,
        confidence: 0.95,
        posterior: 0.9,
        contributors: [
          { name: "influence.stars", contribution: 3.5 },
          { name: "influence.forks", contribution: 2.8 },
          { name: "influence.followers", contribution: 2.2 },
          { name: "influence.total_stars", contribution: 1.9 },
        ],
        breakdown: {
          shipping: 0.9,
          quality: 0.85,
          influence: 0.9,
          complexity: 0.7,
          collaboration: 0.6,
          reliability: 0.95,
          novelty: 0.5,
        },
      };

      mockAnalyze.mockResolvedValue(mockResult);

      render(() => <App />);

      const input = screen.getByPlaceholderText(/e.g., github.com\/user\/repo or @username/);
      const button = screen.getByRole("button", { name: "Analyze" });

      fireEvent.input(input, { target: { value: "facebook/react" } });
      fireEvent.click(button);

      await waitFor(() => {
        expect(screen.getByText("Top Contributors")).toBeInTheDocument();
        expect(screen.getByText("influence.stars")).toBeInTheDocument();
        expect(screen.getByText("+3.5pts")).toBeInTheDocument();
        expect(screen.getByText("influence.forks")).toBeInTheDocument();
        expect(screen.getByText("+2.8pts")).toBeInTheDocument();
        expect(screen.getByText("influence.followers")).toBeInTheDocument();
        expect(screen.getByText("+2.2pts")).toBeInTheDocument();
      });

      // Should only show top 3 contributors
      expect(screen.queryByText("influence.total_stars")).not.toBeInTheDocument();
    });

    it("does not display contributors section when empty", async () => {
      const mockResult = {
        score: 70,
        confidence: 0.75,
        posterior: 0.7,
        contributors: [],
        breakdown: {
          shipping: 0.5,
          quality: 0.6,
          influence: 0.7,
          complexity: 0.4,
          collaboration: 0.3,
          reliability: 0.75,
          novelty: 0.2,
        },
      };

      mockAnalyze.mockResolvedValue(mockResult);

      render(() => <App />);

      const input = screen.getByPlaceholderText(/e.g., github.com\/user\/repo or @username/);
      const button = screen.getByRole("button", { name: "Analyze" });

      fireEvent.input(input, { target: { value: "facebook/react" } });
      fireEvent.click(button);

      await waitFor(() => {
        expect(screen.queryByText("Top Contributors")).not.toBeInTheDocument();
      });
    });
  });

  describe("Error Handling", () => {
    it("handles API errors gracefully", async () => {
      mockAnalyze.mockRejectedValue(new Error("Repository not found"));

      render(() => <App />);

      const input = screen.getByPlaceholderText(/e.g., github.com\/user\/repo or @username/);
      const button = screen.getByRole("button", { name: "Analyze" });

      fireEvent.input(input, { target: { value: "nonexistent/repo" } });
      fireEvent.click(button);

      await waitFor(() => {
        expect(screen.getByText("Something went wrong")).toBeInTheDocument();
        expect(screen.getByText("Repository not found")).toBeInTheDocument();
      });
    });

    it("allows retry after error", async () => {
      mockAnalyze.mockRejectedValueOnce(new Error("Network error")).mockResolvedValueOnce({
        score: 80,
        confidence: 0.85,
        posterior: 0.8,
        contributors: [],
        breakdown: {
          shipping: 0.7,
          quality: 0.75,
          influence: 0.8,
          complexity: 0.6,
          collaboration: 0.5,
          reliability: 0.85,
          novelty: 0.4,
        },
      });

      render(() => <App />);

      const input = screen.getByPlaceholderText(/e.g., github.com\/user\/repo or @username/);
      const button = screen.getByRole("button", { name: "Analyze" });

      // First attempt - error
      fireEvent.input(input, { target: { value: "facebook/react" } });
      fireEvent.click(button);

      await waitFor(() => {
        expect(screen.getByText("Network error")).toBeInTheDocument();
      });

      // Click retry
      const retryButton = screen.getByText("Try Again");
      fireEvent.click(retryButton);

      // Second attempt - success
      await waitFor(() => {
        expect(screen.getByText("80/100")).toBeInTheDocument();
      });
    });

    it("clears previous results when starting new analysis", async () => {
      const firstResult = {
        score: 70,
        confidence: 0.75,
        posterior: 0.7,
        contributors: [{ name: "influence.stars", contribution: 2.1 }],
        breakdown: {
          shipping: 0.5,
          quality: 0.6,
          influence: 0.7,
          complexity: 0.4,
          collaboration: 0.3,
          reliability: 0.75,
          novelty: 0.2,
        },
      };

      const secondResult = {
        score: 90,
        confidence: 0.95,
        posterior: 0.9,
        contributors: [{ name: "influence.forks", contribution: 3.2 }],
        breakdown: {
          shipping: 0.8,
          quality: 0.85,
          influence: 0.9,
          complexity: 0.7,
          collaboration: 0.6,
          reliability: 0.95,
          novelty: 0.5,
        },
      };

      mockAnalyze.mockResolvedValueOnce(firstResult).mockResolvedValueOnce(secondResult);

      render(() => <App />);

      const input = screen.getByPlaceholderText(/e.g., github.com\/user\/repo or @username/);
      const button = screen.getByRole("button", { name: "Analyze" });

      // First analysis
      fireEvent.input(input, { target: { value: "facebook/react" } });
      fireEvent.click(button);

      await waitFor(() => {
        expect(screen.getByText("70/100")).toBeInTheDocument();
        expect(screen.getByText("influence.stars")).toBeInTheDocument();
      });

      // Second analysis
      fireEvent.input(input, { target: { value: "octocat" } });
      fireEvent.click(button);

      await waitFor(() => {
        expect(screen.getByText("90/100")).toBeInTheDocument();
        expect(screen.queryByText("influence.stars")).not.toBeInTheDocument();
        expect(screen.getByText("influence.forks")).toBeInTheDocument();
      });
    });
  });

  describe("Accessibility", () => {
    it("has proper form labels", () => {
      render(() => <App />);

      const input = screen.getByPlaceholderText(/e.g., github.com\/user\/repo or @username/);
      expect(input).toHaveAttribute("placeholder", expect.stringContaining("github.com"));
    });

    it("handles keyboard navigation", () => {
      render(() => <App />);

      const input = screen.getByPlaceholderText(/e.g., github.com\/user\/repo or @username/);
      const button = screen.getByRole("button", { name: "Analyze" });

      // Focus input
      input.focus();
      expect(document.activeElement).toBe(input);

      // Tab to button
      fireEvent.keyDown(input, { key: "Tab" });
      expect(document.activeElement).toBe(button);
    });
  });

  describe("Edge Cases", () => {
    it("handles very long input gracefully", () => {
      render(() => <App />);

      const longInput = "a".repeat(1000);
      const input = screen.getByPlaceholderText(/e.g., github.com\/user\/repo or @username/);

      fireEvent.input(input, { target: { value: longInput } });
      expect(input).toHaveValue(longInput);
    });

    it("handles special characters in input", () => {
      render(() => <App />);

      const specialInput = "user-name_123/repo@branch";
      const input = screen.getByPlaceholderText(/e.g., github.com\/user\/repo or @username/);

      fireEvent.input(input, { target: { value: specialInput } });
      expect(input).toHaveValue(specialInput);
    });

    it("handles multiple rapid submissions", async () => {
      mockAnalyze.mockResolvedValue({
        score: 75,
        confidence: 0.8,
        posterior: 0.75,
        contributors: [],
        breakdown: {
          shipping: 0.6,
          quality: 0.7,
          influence: 0.75,
          complexity: 0.5,
          collaboration: 0.4,
          reliability: 0.8,
          novelty: 0.3,
        },
      });

      render(() => <App />);

      const input = screen.getByPlaceholderText(/e.g., github.com\/user\/repo or @username/);
      const button = screen.getByRole("button", { name: "Analyze" });

      fireEvent.input(input, { target: { value: "facebook/react" } });

      // Rapid clicks
      fireEvent.click(button);
      fireEvent.click(button);
      fireEvent.click(button);

      // Should only call analyze once due to loading state
      expect(mockAnalyze).toHaveBeenCalledTimes(1);
    });
  });
});
