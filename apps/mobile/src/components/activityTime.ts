export function formatActivityTime(value: string, now = new Date()): string {
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return ''
  const year = date.getFullYear()
  const monthDay = `${date.getMonth() + 1}月${date.getDate()}日`
  if (year !== now.getFullYear()) return `${year}年${monthDay}`
  // Calendar-day arithmetic avoids daylight-saving changes affecting day boundaries.
  const dayNumber = (d: Date) => Date.UTC(d.getFullYear(), d.getMonth(), d.getDate()) / 86400000
  const daysAgo = dayNumber(now) - dayNumber(date)
  const time = `${String(date.getHours()).padStart(2, '0')}:${String(date.getMinutes()).padStart(2, '0')}`
  if (daysAgo === 0) return time
  if (daysAgo === 1) return `昨天${time}`
  const daysSinceMonday = (now.getDay() + 6) % 7
  if (daysAgo > 1 && daysAgo <= daysSinceMonday) return `星期${'日一二三四五六'[date.getDay()]}`
  return monthDay
}
