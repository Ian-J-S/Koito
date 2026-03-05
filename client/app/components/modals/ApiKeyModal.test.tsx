import React from "react";
import { render, screen } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { test, expect, vi, beforeEach, afterEach } from "vitest";
import "@testing-library/jest-dom/vitest";

import ApiKeysModal from "./ApiKeysModal";

function renderWithQueryClient(ui: React.ReactElement) {
  const queryClient = new QueryClient({
    defaultOptions: {
      queries: {
        retry: false,
      },
    },
  });

  return render(
    <QueryClientProvider client={queryClient}>{ui}</QueryClientProvider>
  );
}

beforeEach(() => {
  vi.restoreAllMocks();
});

afterEach(() => {
  vi.unstubAllGlobals();
});

test("displays error when API returns HTTP 500 and make sure to stops loading", async () => {
    vi.stubGlobal(
    "fetch",
    vi.fn(async () => ({
        ok: false,
        status: 500,
        text: async () => "Internal Error",
        json: async () => {
        throw new Error("not json");
        },
    })) as any
    );

    renderWithQueryClient(<ApiKeysModal />);

    const err = await screen.findByText(/error:/i);
    expect(err).toBeInTheDocument();
    expect(err.textContent?.toLowerCase()).toContain("not json");

    // 3) Loading should not remain forever
    expect(screen.queryByRole("status")).not.toBeInTheDocument();
});