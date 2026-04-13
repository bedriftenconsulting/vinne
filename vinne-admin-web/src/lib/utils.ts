import { type ClassValue, clsx } from 'clsx'
import { twMerge } from 'tailwind-merge'
import { AxiosError } from 'axios'

export function cn(...inputs: ClassValue[]) {
  return twMerge(clsx(inputs))
}

export function getErrorMessage(error: unknown): string {
  if (error instanceof AxiosError) {
    // Check for our API's standard error response format
    if (error.response?.data?.message) {
      return error.response.data.message
    }

    // Check for simple error format
    if (error.response?.data?.error) {
      return error.response.data.error
    }

    // Check for validation errors array
    if (error.response?.data?.errors && Array.isArray(error.response.data.errors)) {
      return error.response.data.errors.join(', ')
    }

    // Fallback to HTTP status text
    if (error.response?.status) {
      return `${error.response.status}: ${error.response.statusText || 'Request failed'}`
    }

    // Network error or no response
    if (error.code === 'ECONNREFUSED' || error.code === 'NETWORK_ERROR') {
      return 'Unable to connect to server. Please check your connection.'
    }

    return error.message || 'Network error occurred'
  }

  // Handle non-Axios errors
  if (error instanceof Error) {
    return error.message
  }

  // Fallback for unknown error types
  return 'An unexpected error occurred'
}

export function formatCurrency(amount: number): string {
  // Convert from pesewas to GHS and format
  const ghs = amount / 100
  return new Intl.NumberFormat('en-GH', {
    style: 'currency',
    currency: 'GHS',
    minimumFractionDigits: 2,
  }).format(ghs)
}
