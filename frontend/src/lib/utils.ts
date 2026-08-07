// Utility functions (e.g. date formatting)

/**
 * Formats an ISO date string into a user-friendly time format.
 * - YYYY-MM-DD HH:MM for year-month-day hours:minutes
 * @param dateString The ISO 8601 date string to format.
 */
export function formatDateTime(isoString: string) {
  const date = new Date(isoString);
  const pad = (num: number) => num.toString().padStart(2, "0");

  const year = date.getFullYear();
  const month = pad(date.getMonth() + 1);
  const day = pad(date.getDate());
  const hours = pad(date.getHours());
  const minutes = pad(date.getMinutes());

  return `${year}-${month}-${day} ${hours}:${minutes}`;
}

/**
 * Formats an ISO date string lastActivityAt into a user-friendly relative time format.
 * - HH:MM for today
 * - MM:DD for this year
 * - Xyr for previous years
 * @param dateString The ISO 8601 date string to format.
 */
export function formatLastActivity(dateString: string): string {
  const now = new Date();
  const activityDate = new Date(dateString);
  const diffTime = now.getTime() - activityDate.getTime();
  const diffDays = diffTime / (1000 * 60 * 60 * 24);

  if (now.getFullYear() === activityDate.getFullYear()) {
    if (
      now.getMonth() === activityDate.getMonth() &&
      now.getDate() === activityDate.getDate()
    ) {
      // Today: Show HH:MM
      return activityDate.toLocaleTimeString("en-US", {
        hour: "2-digit",
        minute: "2-digit",
        hour12: false,
      });
    }
    // This year: Show MM:DD
    const month = (activityDate.getMonth() + 1).toString().padStart(2, "0");
    const day = activityDate.getDate().toString().padStart(2, "0");
    return `${month}-${day}`;
  }

  // Previous years
  const diffYears = Math.floor(diffDays / 365);
  return `${diffYears}yr`;
}
