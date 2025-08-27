import { beforeAll, beforeEach, vi } from 'vitest'
import '@testing-library/jest-dom'

// Mock fetch globally
const fetchMock = vi.fn()
vi.stubGlobal('fetch', fetchMock)

// Mock requestAnimationFrame
const requestAnimationFrameMock = vi.fn((cb) => setTimeout(cb, 16))
vi.stubGlobal('requestAnimationFrame', requestAnimationFrameMock)

// Mock Date.now for consistent timing in tests
const originalDateNow = Date.now
let mockTime = 1000000000000 // Fixed timestamp for tests

beforeAll(() => {
    vi.stubGlobal('Date', {
        ...Date,
        now: () => mockTime,
    })

    // Helper to advance mock time
    vi.stubGlobal('advanceTime', (ms: number) => {
        mockTime += ms
    })

    // Reset mock time before each test
    vi.stubGlobal('resetTime', () => {
        mockTime = 1000000000000
    })
})

// Reset all mocks before each test
beforeEach(() => {
    vi.clearAllMocks()
    mockTime = 1000000000000
})

// Global test utilities
declare global {
    function advanceTime(ms: number): void
    function resetTime(): void
}

export { }
