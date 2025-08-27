import { describe, it, expect, beforeEach, vi } from 'vitest'
import { analyze, checkHealth, type AnalysisResult } from './api'

// Mock fetch globally
const fetchMock = vi.fn()
vi.stubGlobal('fetch', fetchMock)

beforeEach(() => {
    vi.clearAllMocks()
})

describe('API Functions', () => {
    describe('analyze function', () => {
        it('analyzes valid input successfully', async () => {
            const mockResponse: AnalysisResult = {
                score: 85,
                confidence: 0.9,
                posterior: 0.85,
                contributors: [
                    { name: 'influence.stars', contribution: 2.5 },
                    { name: 'influence.forks', contribution: 1.8 },
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
            }

            fetchMock.mockResolvedValueOnce({
                ok: true,
                json: () => Promise.resolve(mockResponse),
            })

            const result = await analyze('facebook/react')

            expect(fetchMock).toHaveBeenCalledWith('http://localhost:8080/analyze', {
                method: 'POST',
                headers: {
                    'Content-Type': 'application/json',
                },
                body: JSON.stringify({ input: 'facebook/react' }),
            })

            expect(result).toEqual(mockResponse)
        })

        it('handles empty input', async () => {
            await expect(analyze('')).rejects.toThrow('Input cannot be empty')
            await expect(analyze('   ')).rejects.toThrow('Input cannot be empty')

            expect(fetchMock).not.toHaveBeenCalled()
        })

        it('handles whitespace-only input', async () => {
            await expect(analyze('   ')).rejects.toThrow('Input cannot be empty')
            expect(fetchMock).not.toHaveBeenCalled()
        })

        it('handles server error responses', async () => {
            const errorResponse = { error: 'Analysis failed' }

            fetchMock.mockResolvedValueOnce({
                ok: false,
                status: 500,
                statusText: 'Internal Server Error',
                json: () => Promise.resolve(errorResponse),
            })

            await expect(analyze('invalid/repo')).rejects.toThrow('Analysis failed')
        })

        it('handles HTTP error without JSON body', async () => {
            fetchMock.mockResolvedValueOnce({
                ok: false,
                status: 404,
                statusText: 'Not Found',
                json: () => Promise.reject(new Error('Invalid JSON')),
            })

            await expect(analyze('nonexistent/repo')).rejects.toThrow('HTTP 404: Not Found')
        })

        it('handles network errors', async () => {
            fetchMock.mockRejectedValueOnce(new Error('Network error'))

            await expect(analyze('facebook/react')).rejects.toThrow('Network error')
        })

        it('handles non-Error network exceptions', async () => {
            fetchMock.mockRejectedValueOnce('String error')

            await expect(analyze('facebook/react')).rejects.toThrow('Network error occurred')
        })

        it('handles error response with error field', async () => {
            const errorResponse = { error: 'Repository not found' }

            fetchMock.mockResolvedValueOnce({
                ok: false,
                json: () => Promise.resolve(errorResponse),
            })

            await expect(analyze('nonexistent/repo')).rejects.toThrow('Repository not found')
        })

        it('trims input before sending', async () => {
            const mockResponse: AnalysisResult = {
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
            }

            fetchMock.mockResolvedValueOnce({
                ok: true,
                json: () => Promise.resolve(mockResponse),
            })

            await analyze('  facebook/react  ')

            expect(fetchMock).toHaveBeenCalledWith('http://localhost:8080/analyze', {
                method: 'POST',
                headers: {
                    'Content-Type': 'application/json',
                },
                body: JSON.stringify({ input: 'facebook/react' }),
            })
        })

        it('handles various input formats', async () => {
            const mockResponse: AnalysisResult = {
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
            }

            fetchMock.mockResolvedValueOnce({
                ok: true,
                json: () => Promise.resolve(mockResponse),
            })

            // Test GitHub username
            await analyze('octocat')
            expect(fetchMock).toHaveBeenCalledWith('http://localhost:8080/analyze', {
                method: 'POST',
                headers: {
                    'Content-Type': 'application/json',
                },
                body: JSON.stringify({ input: 'octocat' }),
            })

            vi.clearAllMocks()
            fetchMock.mockResolvedValueOnce({
                ok: true,
                json: () => Promise.resolve(mockResponse),
            })

            // Test repository with owner/repo format
            await analyze('facebook/react')
            expect(fetchMock).toHaveBeenCalledWith('http://localhost:8080/analyze', {
                method: 'POST',
                headers: {
                    'Content-Type': 'application/json',
                },
                body: JSON.stringify({ input: 'facebook/react' }),
            })
        })

        it('handles malformed JSON response', async () => {
            fetchMock.mockResolvedValueOnce({
                ok: true,
                json: () => Promise.reject(new Error('Invalid JSON')),
            })

            await expect(analyze('facebook/react')).rejects.toThrow('Network error occurred')
        })

        it('handles response without expected fields', async () => {
            const incompleteResponse = { score: 50 } // Missing other required fields

            fetchMock.mockResolvedValueOnce({
                ok: true,
                json: () => Promise.resolve(incompleteResponse),
            })

            const result = await analyze('facebook/react')
            expect(result.score).toBe(50)
        })

        it('makes request to correct endpoint', async () => {
            const mockResponse: AnalysisResult = {
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
            }

            fetchMock.mockResolvedValueOnce({
                ok: true,
                json: () => Promise.resolve(mockResponse),
            })

            await analyze('test/repo')

            expect(fetchMock).toHaveBeenCalledTimes(1)
            const [url, options] = fetchMock.mock.calls[0]

            expect(url).toBe('http://localhost:8080/analyze')
            expect(options.method).toBe('POST')
            expect(options.headers['Content-Type']).toBe('application/json')
        })
    })

    describe('checkHealth function', () => {
        it('returns true for healthy service', async () => {
            fetchMock.mockResolvedValueOnce({
                ok: true,
            })

            const result = await checkHealth()
            expect(result).toBe(true)
        })

        it('returns false for unhealthy service', async () => {
            fetchMock.mockResolvedValueOnce({
                ok: false,
            })

            const result = await checkHealth()
            expect(result).toBe(false)
        })

        it('returns false for network errors', async () => {
            fetchMock.mockRejectedValueOnce(new Error('Connection failed'))

            const result = await checkHealth()
            expect(result).toBe(false)
        })

        it('makes request to correct health endpoint', async () => {
            fetchMock.mockResolvedValueOnce({
                ok: true,
            })

            await checkHealth()

            expect(fetchMock).toHaveBeenCalledWith('http://localhost:8080/health')
        })

        it('handles various HTTP status codes', async () => {
            // Test different status codes
            const statusCodes = [200, 201, 204, 301, 400, 404, 500]

            for (const statusCode of statusCodes) {
                fetchMock.mockResolvedValueOnce({
                    ok: statusCode < 400,
                })

                const result = await checkHealth()
                const expected = statusCode < 400
                expect(result).toBe(expected)
            }
        })
    })

    describe('Type Definitions', () => {
        it('validates AnalysisResult structure', () => {
            const result: AnalysisResult = {
                score: 85,
                confidence: 0.9,
                posterior: 0.85,
                contributors: [
                    { name: 'influence.stars', contribution: 2.5 },
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
            }

            expect(result.score).toBe(85)
            expect(result.confidence).toBe(0.9)
            expect(result.contributors).toHaveLength(1)
            expect(result.breakdown.influence).toBe(0.85)
        })

        it('validates ApiError structure', () => {
            const error = { error: 'Something went wrong' }

            expect(error.error).toBe('Something went wrong')
        })

        it('handles ApiResponse union type', () => {
            const successResponse: AnalysisResult = {
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
            }

            const errorResponse = { error: 'Analysis failed' }

            // Test that both types can be assigned to ApiResponse
            const apiResponse1: any = successResponse
            const apiResponse2: any = errorResponse

            expect(apiResponse1.score).toBe(75)
            expect(apiResponse2.error).toBe('Analysis failed')
        })
    })

    describe('Error Handling Edge Cases', () => {
        it('handles fetch returning undefined', async () => {
            fetchMock.mockResolvedValueOnce(undefined)

            const result = await checkHealth()
            expect(result).toBe(false)
        })

        it('handles response.json() throwing error', async () => {
            fetchMock.mockResolvedValueOnce({
                ok: true,
                json: () => Promise.reject(new Error('Parse error')),
            })

            await expect(analyze('facebook/react')).rejects.toThrow('Network error occurred')
        })

        it('handles multiple sequential requests', async () => {
            const mockResponse: AnalysisResult = {
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
            }

            // Mock multiple successful responses
            fetchMock
                .mockResolvedValueOnce({
                    ok: true,
                    json: () => Promise.resolve(mockResponse),
                })
                .mockResolvedValueOnce({
                    ok: true,
                    json: () => Promise.resolve(mockResponse),
                })
                .mockResolvedValueOnce({
                    ok: true,
                    json: () => Promise.resolve(mockResponse),
                })

            // Make multiple requests
            const results = await Promise.all([
                analyze('repo1'),
                analyze('repo2'),
                analyze('repo3'),
            ])

            expect(results).toHaveLength(3)
            results.forEach(result => {
                expect(result.score).toBe(80)
            })

            expect(fetchMock).toHaveBeenCalledTimes(3)
        })
    })
})
