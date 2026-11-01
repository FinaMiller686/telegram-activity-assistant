export type CalendarEvent = { id: number; nameZh: string; nameEn: string; summary: string; registrationUrl: string; location: string; floor: string; room: string; warnings?: string[]; sources: { link: string; deleted: boolean }[] };
export function singaporeInput(value: string | null): string;
export function createCalendar(event: CalendarEvent, start: string, end: string, now?: Date): string;
export function createCalendarURL(event: CalendarEvent, start: string, end: string, provider: "google" | "outlook" | "office365"): string;
