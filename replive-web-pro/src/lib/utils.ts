import { type ClassValue, clsx } from "clsx";
import { twMerge } from "tailwind-merge";

export function cn(...inputs: ClassValue[]) {
  return twMerge(clsx(inputs));
}

function localDateTimeParts(date: Date): Record<string, string> {
  return Object.fromEntries(
    new Intl.DateTimeFormat("en-CA", {
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
    const { year, month, day } = localDateTimeParts(date);
    return [year, month, day].join("-");
  } catch {
    return dateStr.slice(0, 10);
  }
}

export function currentLocalCalendarDate(): Date {
  const { year, month, day } = localDateTimeParts(new Date());
  return new Date(Number(year), Number(month) - 1, Number(day));
}

export function formatTimeStr(dateStr: string): string {
  try {
    const date = new Date(dateStr);
    if (Number.isNaN(date.getTime())) return dateStr;
    const { year, month, day, hour, minute, second } = localDateTimeParts(date);
    return [year, month, day].join("-") + " " + [hour, minute, second].join(":");
  } catch {
    return dateStr;
  }
}

export function formatShortDate(dateStr: string): string {
  try {
    const date = new Date(dateStr);
    if (Number.isNaN(date.getTime())) return dateStr;
    const { month, day } = localDateTimeParts(date);
    return String(Number(month)) + "/" + day;
  } catch {
    return dateStr;
  }
}

export function formatWeekday(dateStr: string): string {
  try {
    const normalizedStr = /^\d{4}-\d{2}-\d{2}$/.test(dateStr.trim())
      ? `${dateStr.trim()}T12:00:00`
      : dateStr;
    const date = new Date(normalizedStr);
    if (Number.isNaN(date.getTime())) return "";
    return new Intl.DateTimeFormat("zh-CN", {
      weekday: "long",
    }).format(date);
  } catch {
    return "";
  }
}
