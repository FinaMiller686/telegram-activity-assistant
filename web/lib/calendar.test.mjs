import test from "node:test";
import assert from "node:assert/strict";
import { createCalendar, createCalendarURL, singaporeInput } from "./calendar.mjs";

const event = {id: 12, nameZh: "研究讲座", nameEn: "Research Seminar", summary: "讲座介绍", registrationUrl: "https://example.org/form", location: "Hall A", floor: "2", room: "101", sources: [{link:"https://t.me/example/123",deleted:false}]};
test("calendar files explicitly carry Singapore local times across midnight", () => {
  const result = createCalendar(event, "2026-10-29T23:30", "2026-10-30T01:00", new Date("2026-10-01T00:00:00Z"));
  assert.match(result, /DTSTART;TZID=Asia\/Singapore:20261029T233000\r\nDTEND;TZID=Asia\/Singapore:20261030T010000/);
  assert.match(result, /BEGIN:VTIMEZONE\r\nTZID:Asia\/Singapore/);
  assert.match(result, /TZOFFSETFROM:\+0800\r\nTZOFFSETTO:\+0800\r\nTZNAME:SGT/);
  assert.match(result, /DTSTAMP:20261001T000000Z/);
  assert.match(result, /SUMMARY:研究讲座/);
  const unfolded = result.replace(/\r\n[ \t]/g, "");
  assert.match(unfolded, /https:\/\/example\.org\/form/);
  assert.match(unfolded, /https:\/\/t\.me\/example\/123/);
  assert.equal(singaporeInput("2026-10-29T15:30:00Z"), "2026-10-29T23:30");
});
test("missing, invalid, or reversed times are not guessed", () => {
  for (const [a, b] of [["", ""], ["2026-02-30T10:00", "2026-03-01T11:00"], ["2026-10-29T11:00", "2026-10-29T10:00"]]) assert.throws(() => createCalendar(event,a,b));
});
test("Singapore export keeps early-morning and New Year dates", () => {
  for (const [start, end, expectedStart, expectedEnd] of [
    ["2026-11-01T00:15", "2026-11-01T01:00", "20261101T001500", "20261101T010000"],
    ["2026-12-31T23:30", "2027-01-01T01:00", "20261231T233000", "20270101T010000"],
  ]) {
    const result = createCalendar(event, start, end);
    assert.ok(result.includes(`DTSTART;TZID=Asia/Singapore:${expectedStart}`));
    assert.ok(result.includes(`DTEND;TZID=Asia/Singapore:${expectedEnd}`));
  }
});
test("text cannot inject calendar properties and UTF-8 lines are folded", () => {
  const result = createCalendar({...event, nameZh: "讲座\r\nATTENDEE:other@example.com".repeat(12)}, "2026-10-29T10:00", "2026-10-29T11:00");
  assert.doesNotMatch(result, /\r\nATTENDEE:/);
  for (const line of result.split("\r\n")) assert.ok(new TextEncoder().encode(line).length <= 75);
});

test("Google and both Outlook account types receive exact UTC times and encoded fields", () => {
  const value = {...event, nameZh: "研究 & 招生 + #1", warnings: ["年份需要核对"]};
  const google = new URL(createCalendarURL(value, "2026-10-29T23:30", "2026-10-30T01:00", "google"));
  assert.equal(google.origin, "https://calendar.google.com");
  assert.equal(google.searchParams.get("dates"), "20261029T153000Z/20261029T170000Z");
  assert.equal(google.searchParams.get("text"), value.nameZh);
  assert.equal(google.searchParams.get("ctz"), "Asia/Singapore");
  assert.match(google.searchParams.get("details"), /年份需要核对/);
  assert.match(google.searchParams.get("details"), /https:\/\/t.me\/example\/123/);
  for (const [provider, host] of [["outlook", "outlook.live.com"], ["office365", "outlook.office.com"]]) {
    const url = new URL(createCalendarURL(value, "2026-10-29T23:30", "2026-10-30T01:00", provider));
    assert.equal(url.hostname, host);
    assert.equal(url.searchParams.get("subject"), value.nameZh);
    assert.equal(url.searchParams.get("startdt"), "2026-10-29T15:30:00Z");
    assert.equal(url.searchParams.get("enddt"), "2026-10-29T17:00:00Z");
    assert.equal(url.searchParams.get("location"), "Hall A 2 101");
  }
});

test("calendar drafts reject unknown times and preserve dates when shortening long descriptions", () => {
  for (const provider of ["google", "outlook", "office365"]) {
    assert.throws(() => createCalendarURL(event, "", "", provider));
    assert.throws(() => createCalendarURL(event, "2026-02-30T10:00", "2026-03-01T11:00", provider));
    assert.throws(() => createCalendarURL(event, "2026-10-29T11:00", "2026-10-29T10:00", provider));
    const result = createCalendarURL({...event, summary: "很长的内容".repeat(2000), warnings: ["请核对年份"]}, "2026-10-29T10:00", "2026-10-29T11:00", provider);
    assert.ok(result.length <= 2000);
    assert.match(decodeURIComponent(result.replace(/\+/g, " ")), /请核对年份/);
    assert.throws(() => createCalendarURL({...event, nameZh: "长".repeat(3000)}, "2026-10-29T10:00", "2026-10-29T11:00", provider), /calendar_link_too_long/);
  }
  assert.throws(() => createCalendarURL(event, "2026-10-29T10:00", "2026-10-29T11:00", "untrusted"));
});

test("draft text cannot create parameters or active Outlook markup", () => {
  const value = {...event, nameZh: "Title&attendees=other", summary: '<img src="https://invalid.example/track">\n<script>bad()</script>', sources: [{link: "javascript:alert(1)", deleted: false}]};
  const url = new URL(createCalendarURL(value, "2026-10-29T10:00", "2026-10-29T11:00", "outlook"));
  assert.equal(url.searchParams.get("attendees"), null);
  assert.doesNotMatch(url.searchParams.get("body"), /<img|<script|javascript:/);
  assert.match(url.searchParams.get("body"), /&lt;img/);
});
