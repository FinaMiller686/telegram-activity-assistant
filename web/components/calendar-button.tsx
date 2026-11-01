"use client";

import { useEffect, useState } from "react";
import { Button, Field, Input } from "@/components/ui";
import { createCalendar, createCalendarURL, singaporeInput, type CalendarEvent } from "@/lib/calendar.mjs";

export function CalendarButton({event}: {event: CalendarEvent & {start: string | null; end: string | null}}) {
  const [start, setStart] = useState(() => singaporeInput(event.start));
  const [end, setEnd] = useState(() => singaporeInput(event.end));
  const [expanded, setExpanded] = useState(false);
  const [message, setMessage] = useState("");
  useEffect(() => { setStart(singaporeInput(event.start)); setEnd(singaporeInput(event.end)); }, [event.start, event.end]);

  function download() {
    if (!start || !end) {
      setExpanded(true);
      setMessage("请填写开始和结束时间，程序不会猜测时间。");
      return;
    }
    try {
      const data = createCalendar(event, start, end);
      const url = URL.createObjectURL(new Blob([data], {type: "text/calendar;charset=utf-8"}));
      const link = document.createElement("a");
      link.href = url;
      link.download = `activity-${event.id}.ics`;
      document.body.appendChild(link);
      link.click();
      link.remove();
      setTimeout(() => URL.revokeObjectURL(url), 60000);
      setMessage("已导出日历文件，时间按新加坡（UTC+8）。");
    } catch {
      setExpanded(true);
      setMessage("请检查日期和时间，结束时间须晚于开始时间。");
    }
  }
  function openCalendar(provider: "google" | "outlook" | "office365") {
    try {
      const url = createCalendarURL(event, start, end, provider);
      window.open(url, "_blank", "noopener,noreferrer");
      setMessage("请在打开的日历页面核对内容并保存。");
    } catch (error) {
      setExpanded(true);
      setMessage(error instanceof Error && error.message === "calendar_link_too_long"
        ? "内容较长，请改用 .ics 文件导入。"
        : "请检查日期和时间，结束时间须晚于开始时间。");
    }
  }
  return <div className="activity-calendar">
    <div className="activity-calendar-actions">
    <Button onClick={download}>导出 .ics 日历文件</Button>{" "}
    <Button onClick={()=>openCalendar("google")}>Google Calendar</Button>{" "}
    <Button onClick={()=>openCalendar("office365")}>Outlook（学校/工作）</Button>{" "}
    <Button onClick={()=>openCalendar("outlook")}>Outlook.com（个人）</Button>{" "}
    <button type="button" onClick={() => setExpanded(!expanded)} aria-expanded={expanded}>填写或修改时间</button>
    </div>
    {expanded && <div className="activity-filters">
      <Field label="日历开始时间（新加坡）"><Input type="datetime-local" value={start} onChange={e=>setStart(e.target.value)} /></Field>
      <Field label="日历结束时间（新加坡）"><Input type="datetime-local" value={end} onChange={e=>setEnd(e.target.value)} /></Field>
    </div>}
    {message && <p role="status">{message}</p>}
    <p>请在日历中核对内容并确认保存。</p>
  </div>;
}
