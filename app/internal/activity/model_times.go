package activity

import (
	"regexp"
	"strings"
	"time"
)

type modelTimes struct {
	Date, Start, End, Deadline, DeadlineDate     string
	DateEvidence, TimeEvidence, DeadlineEvidence string
}

var modelDateSignal = regexp.MustCompile(`(?i)\b(?:` + monthNames + `|today|tomorrow|tonight|Monday|Tuesday|Wednesday|Thursday|Friday|Saturday|Sunday|Mon|Tue|Wed|Thu|Fri|Sat|Sun)\b|\d{1,4}[/.-]\d{1,2}|今天|明天|星期|周[一二三四五六日天]|\d月\d`)
var modelClockSignal = regexp.MustCompile(`(?i)\d\s*(?:am|pm)\b|\d[:.]\d{2}|\b\d{3,4}\s*(?:h|hrs|hours)\b|noon|midnight|上午|下午|晚上|点|时`)
var explicitYear = regexp.MustCompile(`\b(?:19|20)\d{2}\b`)
var foreignZone = regexp.MustCompile(`(?i)\b(?:UTC|GMT|EST|EDT|PST|PDT|BST|CET|CEST)\b`)

func sourceEvidence(text, evidence string) bool {
	clean := func(s string) string { return strings.ToLower(strings.Join(strings.Fields(s), " ")) }
	return strings.TrimSpace(evidence) != "" && len(evidence) <= 1500 && strings.Contains(clean(text), clean(evidence))
}

// Model dates require a real source quote. Relative dates are checked against
// publication time, never the time this worker happens to run.
func groundedModelDate(value, evidence, text string, published time.Time) (string, bool) {
	normalized := regexp.MustCompile(`(?i)(\d)(st|nd|rd|th)`).ReplaceAllString(evidence, "${1}")
	normalized = regexp.MustCompile(`(\d)([A-Za-z])`).ReplaceAllString(normalized, "${1} ${2}")
	normalized = regexp.MustCompile(`([A-Za-z])(\d)`).ReplaceAllString(normalized, "${1} ${2}")
	if !sourceEvidence(text, evidence) || !modelDateSignal.MatchString(normalized) {
		return "", false
	}
	date, err := time.ParseInLocation("2006-01-02", value, Singapore)
	if err != nil {
		return "", false
	}
	if expected, err := ParseDate(evidence, published); err == nil {
		if expected.Format("2006-01-02") != value {
			if !foreignZone.MatchString(text) || date.Sub(*expected) > 24*time.Hour || date.Sub(*expected) < -24*time.Hour {
				return "", false
			}
		}
		return value, false
	}
	weekdayNames := map[string]time.Weekday{"sun": time.Sunday, "mon": time.Monday, "tue": time.Tuesday, "wed": time.Wednesday, "thu": time.Thursday, "fri": time.Friday, "sat": time.Saturday}
	weekday := regexp.MustCompile(`(?i)\b(sun(?:day)?|mon(?:day)?|tue(?:s(?:day)?)?|wed(?:nesday)?|thu(?:r(?:s(?:day)?)?)?|fri(?:day)?|sat(?:urday)?)\b`).FindString(evidence)
	if weekday != "" && !foreignZone.MatchString(text) && weekdayNames[strings.ToLower(weekday[:3])] != date.Weekday() {
		return "", false
	}
	if explicitYear.MatchString(evidence) {
		return value, false
	}
	anchor := published.In(Singapore)
	year := anchor.Year()
	if anchor.Month() == time.December && date.Month() == time.January {
		year++
	}
	if date.Year() != year {
		return "", false
	}
	return value, true
}

func parseModelTimestamp(value string) *time.Time {
	for _, layout := range []string{time.RFC3339, "2006-01-02T15:04Z07:00", "2006-01-02T15:04:05", "2006-01-02T15:04"} {
		if t, err := time.ParseInLocation(layout, value, Singapore); err == nil {
			t = t.In(Singapore)
			return &t
		}
	}
	return nil
}

func applyModelTimes(base *Details, dto modelTimes, text string, published time.Time) {
	inferred := false
	if value, yearInferred := groundedModelDate(dto.Date, dto.DateEvidence, text, published); value != "" {
		if base.Date != value {
			base.Start = nil
			base.End = nil
		}
		base.Date = value
		inferred = yearInferred
	}
	if sourceEvidence(text, dto.DateEvidence) {
		base.DateText = strings.TrimSpace(dto.DateEvidence)
	}
	if sourceEvidence(text, dto.TimeEvidence) {
		base.TimeText = strings.TrimSpace(dto.TimeEvidence)
	}
	if sourceEvidence(text, dto.DeadlineEvidence) {
		base.DeadlineText = strings.TrimSpace(dto.DeadlineEvidence)
	}
	if base.Date != "" && sourceEvidence(text, dto.TimeEvidence) && modelClockSignal.MatchString(dto.TimeEvidence) {
		start := parseModelTimestamp(dto.Start)
		if start != nil && start.Format("2006-01-02") == base.Date {
			base.Start = start
		}
		end := parseModelTimestamp(dto.End)
		if end != nil && base.Start != nil && end.After(*base.Start) {
			base.End = end
		}
	}
	deadline := parseModelTimestamp(dto.Deadline)
	date := dto.DeadlineDate
	if date == "" && deadline != nil {
		date = deadline.Format("2006-01-02")
	}
	if value, yearInferred := groundedModelDate(date, dto.DeadlineEvidence, text, published); value != "" {
		base.DeadlineDate = value
		inferred = inferred || yearInferred
		if deadline != nil && deadline.Format("2006-01-02") == value && modelClockSignal.MatchString(dto.DeadlineEvidence) {
			base.Deadline = deadline
		}
	}
	if base.Start != nil && base.End != nil && !base.End.After(*base.Start) {
		base.End = nil
	}
	if inferred {
		base.Warnings = append(base.Warnings, "原文未写年份，按消息发布时间推定，请核对")
	}
}
