package api

import (
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/fr0der1c/tgtldr/app/internal/activity"
	"github.com/fr0der1c/tgtldr/app/internal/config"
	"github.com/fr0der1c/tgtldr/app/internal/httpx"
)

func (r *Router) handleActivities(w http.ResponseWriter, req *http.Request) {
	items, e := activity.New(r.store).List(req.Context())
	if e != nil {
		httpx.Error(w, 500, "activity_database_error")
		return
	}
	result := []activity.Event{}
	q := req.URL.Query()
	for _, v := range items {
		if q.Get("from") != "" && v.Date < q.Get("from") {
			continue
		}
		if q.Get("to") != "" && v.Date > q.Get("to") {
			continue
		}
		if q.Get("category") != "" && v.Category != q.Get("category") {
			continue
		}
		status := q.Get("status")
		if status == "selected" {
			if v.Status != "going" && v.Status != "registered" {
				continue
			}
		} else if status != "" && v.Status != status {
			continue
		}
		if keyword := strings.ToLower(q.Get("q")); keyword != "" && !strings.Contains(strings.ToLower(v.NameEN+v.NameZH+v.Summary+v.Organizer+v.Location), keyword) {
			continue
		}
		result = append(result, v)
	}
	httpx.JSON(w, 200, map[string]any{"items": result})
}
func (r *Router) handleActivityStatus(w http.ResponseWriter, req *http.Request) {
	id, e := strconv.ParseInt(req.PathValue("id"), 10, 64)
	if e != nil || id <= 0 {
		httpx.Error(w, 400, "invalid_id")
		return
	}
	var p struct {
		Status string `json:"status"`
	}
	if httpx.DecodeJSON(req, &p) != nil || !activity.ValidStatus(p.Status) {
		httpx.Error(w, 400, "invalid_status")
		return
	}
	if e = activity.New(r.store).SetStatus(req.Context(), id, p.Status); e != nil {
		httpx.Error(w, 404, "activity_not_found")
		return
	}
	httpx.JSON(w, 200, map[string]any{"status": "ok"})
}
func (r *Router) handleActivityConfig(w http.ResponseWriter, req *http.Request) {
	s := activity.New(r.store)
	if req.Method == "GET" {
		c, e := s.Config(req.Context())
		if e != nil {
			httpx.Error(w, 500, "config_error")
			return
		}
		httpx.JSON(w, 200, c)
		return
	}
	var c activity.Config
	if httpx.DecodeJSON(req, &c) != nil || s.SaveConfig(req.Context(), c) != nil {
		httpx.Error(w, 400, "invalid_config")
		return
	}
	httpx.JSON(w, 200, map[string]string{"status": "ok"})
}
func (r *Router) handleActivityRuntime(w http.ResponseWriter, req *http.Request) {
	rows, e := r.store.Pool.Query(req.Context(), `select name,code,updated_at from activity_runtime order by name`)
	if e != nil {
		httpx.Error(w, 500, "runtime_error")
		return
	}
	defer rows.Close()
	items := []map[string]any{}
	for rows.Next() {
		var name, code string
		var updated time.Time
		if rows.Scan(&name, &code, &updated) != nil {
			httpx.Error(w, 500, "runtime_error")
			return
		}
		items = append(items, map[string]any{"name": name, "code": code, "updatedAt": updated})
	}
	if rows.Err() != nil {
		httpx.Error(w, 500, "runtime_error")
		return
	}
	rows.Close()
	report, e := activity.New(r.store).DeliveryReport(req.Context())
	if e != nil {
		httpx.Error(w, 500, "runtime_error")
		return
	}
	httpx.JSON(w, 200, map[string]any{"items": items, "failedJobs": report.FailedJobs, "storageProtected": config.StorageProtected(), "delivery": report})
}
func (r *Router) handleActivityDigests(w http.ResponseWriter, req *http.Request) {
	rows, e := r.store.Pool.Query(req.Context(), `select key,content,sent_at,created_at from activity_deliveries where key like 'digest:%' order by key desc limit 60`)
	if e != nil {
		httpx.Error(w, 500, "digest_error")
		return
	}
	defer rows.Close()
	items := []map[string]any{}
	for rows.Next() {
		var key, content string
		var sent *time.Time
		var created time.Time
		if rows.Scan(&key, &content, &sent, &created) != nil {
			httpx.Error(w, 500, "digest_error")
			return
		}
		items = append(items, map[string]any{"key": key, "content": content, "sentAt": sent, "createdAt": created})
	}
	if rows.Err() != nil {
		httpx.Error(w, 500, "digest_error")
		return
	}
	httpx.JSON(w, 200, map[string]any{"items": items})
}
func (r *Router) handleActivityRetry(w http.ResponseWriter, req *http.Request) {
	_, e := r.store.Pool.Exec(req.Context(), `update activity_jobs set attempts=0,next_attempt=now() where error_code<>''`)
	if e != nil {
		httpx.Error(w, 500, "retry_error")
		return
	}
	httpx.JSON(w, 200, map[string]string{"status": "ok"})
}
func (r *Router) handleActivityForget(w http.ResponseWriter, req *http.Request) {
	var p struct {
		Confirm string `json:"confirm"`
	}
	if httpx.DecodeJSON(req, &p) != nil || p.Confirm != "DELETE_HISTORY" {
		httpx.Error(w, 400, "confirmation_required")
		return
	}
	if e := r.telegram.StopAndWait(req.Context()); e != nil {
		httpx.Error(w, 500, "listener_stop_failed")
		return
	}
	if e := activity.New(r.store).Forget(req.Context()); e != nil {
		httpx.Error(w, 500, "history_delete_failed")
		return
	}
	entries, e := os.ReadDir(r.mediaDir)
	if e != nil && !os.IsNotExist(e) {
		httpx.Error(w, 500, "media_delete_failed")
		return
	}
	for _, entry := range entries {
		if os.RemoveAll(filepath.Join(r.mediaDir, entry.Name())) != nil {
			httpx.Error(w, 500, "media_delete_failed")
			return
		}
	}
	if accounts, err := r.store.Auth.List(req.Context()); err == nil && config.StorageProtected() {
		for _, account := range accounts {
			if account.Status == "authorized" {
				r.telegram.EnsureListener(account.ID)
			}
		}
	}
	httpx.JSON(w, 200, map[string]string{"status": "ok"})
}
