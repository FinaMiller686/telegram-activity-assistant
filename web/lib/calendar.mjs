// iCalendar files are generated on the device. Times entered here are Singapore time.
const encoder = new TextEncoder();

function escapeText(value) {
  return String(value ?? "").replace(/\r\n|\r/g, "\n").replace(/[\u0000-\u0008\u000b\u000c\u000e-\u001f\u007f]/g, "").replace(/\\/g, "\\\\").replace(/\n/g, "\\n").replace(/,/g, "\\,").replace(/;/g, "\\;");
}

function fold(line) {
  const lines = [];
  let part = "", bytes = 0;
  for (const character of line) {
    const size = encoder.encode(character).length;
    if (bytes + size > 75) { lines.push(part); part = " "; bytes = 1; }
    part += character;
    bytes += size;
  }
  lines.push(part);
  return lines.join("\r\n");
}

function toUTC(value) {
  if (!/^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}$/.test(value)) throw new Error("请输入完整的开始和结束时间。");
  const date = new Date(`${value}:00+08:00`);
  if (!Number.isFinite(date.getTime()) || new Date(date.getTime() + 8 * 3600000).toISOString().slice(0, 16) !== value) throw new Error("请输入有效的新加坡时间。");
  return date;
}

function stamp(date) { return date.toISOString().replace(/[-:]/g, "").replace(/\.\d{3}/, ""); }

function safeURL(value) {
  try { const u = new URL(value); return (u.protocol === "https:" || u.protocol === "http:") && !u.username && !u.password; } catch { return false; }
}

export function singaporeInput(value) {
  if (!value) return "";
  const date = new Date(value);
  return Number.isFinite(date.getTime()) ? new Date(date.getTime() + 8 * 3600000).toISOString().slice(0, 16) : "";
}

export function createCalendar(event, start, end, now = new Date()) {
  if (!Number.isSafeInteger(event.id) || event.id <= 0) throw new Error("活动编号无效。");
  const from = toUTC(start), until = toUTC(end);
  if (until <= from) throw new Error("结束时间必须晚于开始时间。");
  const title = event.nameZh || event.nameEn || "未命名活动";
  const source = event.sources.find(s => !s.deleted && safeURL(s.link));
  const description = [event.nameZh && event.nameEn ? `英文名称：${event.nameEn}` : "", event.summary, safeURL(event.registrationUrl) ? `报名链接：${event.registrationUrl}` : "", source ? `原始消息：${source.link}` : "", ...(event.warnings ?? [])].filter(Boolean).join("\n");
  const location = [event.location, event.floor, event.room].filter(Boolean).join(" ");
  const localStamp = date => singaporeInput(date.toISOString()).replace(/[-:]/g, "") + "00";
  const lines = [
    "BEGIN:VCALENDAR", "VERSION:2.0", "PRODID:-//TGTLDR//Activity Assistant//ZH", "CALSCALE:GREGORIAN",
    "X-WR-TIMEZONE:Asia/Singapore",
    "BEGIN:VTIMEZONE", "TZID:Asia/Singapore", "X-LIC-LOCATION:Asia/Singapore",
    "BEGIN:STANDARD", "DTSTART:19820101T000000", "TZOFFSETFROM:+0800", "TZOFFSETTO:+0800", "TZNAME:SGT", "END:STANDARD", "END:VTIMEZONE",
    "BEGIN:VEVENT", `UID:activity-${event.id}@tgtldr.local`, `DTSTAMP:${stamp(now)}`,
    `DTSTART;TZID=Asia/Singapore:${localStamp(from)}`, `DTEND;TZID=Asia/Singapore:${localStamp(until)}`,
    `SUMMARY:${escapeText(title)}`, `DESCRIPTION:${escapeText(description)}`,
  ];
  if (location) lines.push(`LOCATION:${escapeText(location)}`);
  lines.push("END:VEVENT", "END:VCALENDAR");
  return lines.map(fold).join("\r\n") + "\r\n";
}

// These links open a draft. They do not create an event or access an account.
export function createCalendarURL(event, start, end, provider) {
  if (!Number.isSafeInteger(event.id) || event.id <= 0) throw new Error("calendar_activity_invalid");
  const from = toUTC(start), until = toUTC(end);
  if (until <= from) throw new Error("calendar_time_incomplete");
  const bases = {
    google: "https://calendar.google.com/calendar/render",
    outlook: "https://outlook.live.com/calendar/0/deeplink/compose",
    office365: "https://outlook.office.com/calendar/0/deeplink/compose",
  };
  if (!Object.hasOwn(bases, provider)) throw new Error("calendar_provider_invalid");
  const title = event.nameZh || event.nameEn || "未命名活动";
  const location = [event.location, event.floor, event.room].filter(Boolean).join(" ").trim();
  const source = event.sources.find(s => !s.deleted && safeURL(s.link));
  for (const limit of [500, 200, 80, 0]) {
    const runes = Array.from(event.summary || "");
    const summary = limit ? runes.slice(0, limit).join("") + (runes.length > limit ? "…" : "") : "";
    const body = [summary, ...(event.warnings ?? []), source ? `原始消息：${source.link}` : ""].filter(Boolean).join("\n");
    const url = new URL(bases[provider]);
    if (provider === "google") {
      url.search = new URLSearchParams({action: "TEMPLATE", text: title, dates: `${stamp(from)}/${stamp(until)}`, ctz: "Asia/Singapore", details: body, location}).toString();
    } else {
      const escaped = body.replace(/&/g, "&amp;").replace(/</g, "&lt;").replace(/>/g, "&gt;").replace(/"/g, "&quot;").replace(/'/g, "&#39;").replace(/\r\n|\r|\n/g, "<br>");
      url.search = new URLSearchParams({path: "/calendar/action/compose", rru: "addevent", subject: title, startdt: from.toISOString().replace(/\.000Z$/, "Z"), enddt: until.toISOString().replace(/\.000Z$/, "Z"), body: escaped, location}).toString();
    }
    if (url.href.length <= 2000) return url.href;
  }
  throw new Error("calendar_link_too_long");
}
