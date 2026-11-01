package activity

import (
	"net/url"
	"strings"
	"time"
)

var Singapore = mustLocation()

func mustLocation() *time.Location {
	l, e := time.LoadLocation("Asia/Singapore")
	if e != nil {
		panic(e)
	}
	return l
}

const Unknown = "未知"

var Categories = []string{"校园活动", "学术讲座", "金融与商业活动", "职业发展与实习", "比赛与工作坊", "学生福利", "其他重要通知"}

type Details struct {
	NameEN               string     `json:"nameEn"`
	NameZH               string     `json:"nameZh"`
	Organizer            string     `json:"organizer"`
	Summary              string     `json:"summary"`
	Category             string     `json:"category"`
	Date                 string     `json:"date"`
	DateText             string     `json:"dateText"`
	TimeText             string     `json:"timeText"`
	DeadlineText         string     `json:"deadlineText"`
	Start                *time.Time `json:"start"`
	End                  *time.Time `json:"end"`
	Location             string     `json:"location"`
	Floor                string     `json:"floor"`
	Room                 string     `json:"room"`
	RegistrationURL      string     `json:"registrationUrl"`
	Deadline             *time.Time `json:"deadline"`
	DeadlineDate         string     `json:"deadlineDate"`
	Eligibility          string     `json:"eligibility"`
	DressCode            string     `json:"dressCode"`
	RegistrationRequired *bool      `json:"registrationRequired"`
	UpdateNotice         bool       `json:"updateNotice"`
	Warnings             []string   `json:"warnings"`
}
type Source struct {
	ChatID    int64  `json:"chatId"`
	MessageID int    `json:"messageId"`
	Title     string `json:"title"`
	Link      string `json:"link"`
	Deleted   bool   `json:"deleted"`
}
type Event struct {
	ID int64 `json:"id"`
	Details
	Status    string    `json:"status"`
	Revision  int       `json:"revision"`
	Created   time.Time `json:"createdAt"`
	Updated   time.Time `json:"updatedAt"`
	Sources   []Source  `json:"sources"`
	Conflicts []int64   `json:"conflicts"`
	Withdrawn bool      `json:"withdrawn"`
}
type Config struct {
	AIEnabled     bool       `json:"aiEnabled"`
	DigestTime    string     `json:"digestTime"`
	ReminderHours int        `json:"reminderHours"`
	RetentionDays int        `json:"retentionDays"`
	OwnerID       int64      `json:"ownerId"`
	CollectSince  *time.Time `json:"collectSince"`
}

func ValidURL(s string) bool {
	u, e := url.Parse(s)
	return len(s) <= 2048 && e == nil && (u.Scheme == "https" || u.Scheme == "http") && u.Hostname() != "" && u.User == nil
}
func SourceLink(username string, id int64, msg int, kind ...string) string {
	if username != "" {
		return "https://t.me/" + username + "/" + itoa(int64(msg))
	}
	if id > 0 && (len(kind) == 0 || kind[0] == "supergroup") {
		return "https://t.me/c/" + itoa(id) + "/" + itoa(int64(msg))
	}
	return ""
}
func ValidStatus(s string) bool {
	return strings.Contains("|new|going|later|ignored|registered|attended|", "|"+s+"|") && s != ""
}
func Overlap(a, b Details) bool {
	return a.Start != nil && a.End != nil && b.Start != nil && b.End != nil && a.Start.Before(*b.End) && b.Start.Before(*a.End)
}
