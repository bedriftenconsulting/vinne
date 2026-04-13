import { formatInTimeZone } from 'date-fns-tz'
import { fromUnixTime } from 'date-fns'

/**
 * GMT timezone (UTC+0)
 * Ghana operates in GMT timezone
 */
export const GHANA_TIMEZONE = 'Etc/GMT'

/**
 * Helper to convert various timestamp formats to Date
 * Handles:
 * - ISO 8601 strings (e.g., "2025-10-17T16:00:00Z")
 * - Protobuf timestamp objects ({ seconds: number, nanos?: number })
 * - Date objects (returned as-is)
 * - null/undefined (returns Unix epoch: Jan 1, 1970)
 *
 * Always returns a valid Date object for safe comparisons/calculations
 */
export const protoTimestampToDate = (
  timestamp: { seconds: number; nanos?: number } | string | undefined | Date | null
): Date => {
  // Return Unix epoch for null/undefined
  if (!timestamp) return new Date(0)

  // Return Date objects as-is
  if (timestamp instanceof Date) return timestamp

  // Handle ISO 8601 strings from backend (e.g., "2025-10-17T16:00:00Z")
  if (typeof timestamp === 'string') {
    const date = new Date(timestamp)
    return isNaN(date.getTime()) ? new Date(0) : date
  }

  // Handle protobuf timestamp objects
  if (typeof timestamp === 'object' && 'seconds' in timestamp) {
    const date = fromUnixTime(timestamp.seconds)
    return isNaN(date.getTime()) ? new Date(0) : date
  }

  return new Date(0)
}

/**
 * Format a date/timestamp in GMT (Ghana timezone)
 * This ensures all users see the same time regardless of their browser timezone
 *
 * @param date - Date, timestamp object, or ISO string
 * @param formatString - date-fns format string (e.g., 'PPP p', 'yyyy-MM-dd HH:mm')
 * @returns Formatted date string in GMT, or 'N/A' if date is invalid/missing (Unix epoch)
 *
 * @example
 * formatInGhanaTime(new Date(), 'PPP p') // "January 1, 2025 2:30 PM" (GMT)
 * formatInGhanaTime(new Date(), 'yyyy-MM-dd HH:mm') // "2025-01-01 14:30" (GMT)
 * formatInGhanaTime(undefined, 'PPP p') // "N/A"
 * formatInGhanaTime(null, 'PPP p') // "N/A"
 */
export const formatInGhanaTime = (
  date: Date | string | { seconds: number; nanos?: number } | undefined | null,
  formatString: string
): string => {
  const dateObj = protoTimestampToDate(date)
  // Check if date is Unix epoch (0) - indicates invalid/missing date
  if (dateObj.getTime() === 0) return 'N/A'
  return formatInTimeZone(dateObj, GHANA_TIMEZONE, formatString)
}

/**
 * Get the start of the current week (Sunday) in GMT timezone
 * This ensures all users see the same week regardless of their browser timezone
 * Note: In Ghana, the week starts on Sunday
 *
 * @returns ISO date string (YYYY-MM-DD) for Sunday of the current week in GMT
 *
 * @example
 * // If today is Wednesday, Oct 17, 2025 in GMT
 * getGMTWeekStart() // "2025-10-13" (Sunday of that week)
 */
export const getGMTWeekStart = (): string => {
  // Get current date in GMT
  const nowInGMT = new Date()
  const gmtDateString = formatInTimeZone(nowInGMT, GHANA_TIMEZONE, 'yyyy-MM-dd')
  const gmtDate = new Date(gmtDateString + 'T00:00:00Z')

  // Get day of week (0 = Sunday, 1 = Monday, ..., 6 = Saturday)
  const dayOfWeek = gmtDate.getUTCDay()

  // Calculate days to subtract to get to Sunday
  // If Sunday (0), stay on current day; if Monday (1), go back 1 day, etc.
  const daysToSunday = dayOfWeek

  // Create Sunday date
  const sunday = new Date(gmtDate)
  sunday.setUTCDate(gmtDate.getUTCDate() - daysToSunday)

  // Return in YYYY-MM-DD format
  return formatInTimeZone(sunday, GHANA_TIMEZONE, 'yyyy-MM-dd')
}
