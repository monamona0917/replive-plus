import { type ClassValue, clsx } from "clsx";
import { twMerge } from "tailwind-merge";

export const JAPAN_TIME_ZONE = "Asia/Tokyo";

export function cn(...inputs: ClassValue[]) {
  return twMerge(clsx(inputs));
}

function japanDateTimeParts(date: Date): Record<string, string> {
  return Object.fromEntries(
    new Intl.DateTimeFormat("en-CA", {
      timeZone: JAPAN_TIME_ZONE,
      year: "numeric",
      month: "2-digit",
      day: "2-digit",
      hour: "2-digit",
      minute: "2-digit",
      second: "2-digit",
      hourCycle: "h23",
    })
      .formatToParts(date)
      .filter((part) => part.type !== "literal")
      .map((part) => [part.type, part.value]),
  ) as Record<string, string>;
}

export function formatDateKey(dateStr: string): string {
  try {
    const date = new Date(dateStr);
    if (Number.isNaN(date.getTime())) return dateStr.slice(0, 10);
    const { year, month, day } = japanDateTimeParts(date);
    return [year, month, day].join("-");
  } catch {
    return dateStr.slice(0, 10);
  }
}

export function currentJapanCalendarDate(): Date {
  const { year, month, day } = japanDateTimeParts(new Date());
  return new Date(Number(year), Number(month) - 1, Number(day));
}

export function formatTimeStr(dateStr: string): string {
  try {
    const date = new Date(dateStr);
    if (Number.isNaN(date.getTime())) return dateStr;
    const { year, month, day, hour, minute, second } = japanDateTimeParts(date);
    return [year, month, day].join("-") + " " + [hour, minute, second].join(":");
  } catch {
    return dateStr;
  }
}

export function formatShortDate(dateStr: string): string {
  try {
    const date = new Date(dateStr);
    if (Number.isNaN(date.getTime())) return dateStr;
    const { month, day } = japanDateTimeParts(date);
    return String(Number(month)) + "/" + day;
  } catch {
    return dateStr;
  }
}