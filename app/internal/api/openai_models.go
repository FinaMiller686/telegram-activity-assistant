package api

import (
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/fr0der1c/tgtldr/app/internal/activity"
	"github.com/fr0der1c/tgtldr/app/internal/httpx"
	"github.com/fr0der1c/tgtldr/app/internal/model"
	"github.com/fr0der1c/tgtldr/app/internal/openai"
)

func prepareModelEndpoint(payload, current model.AppSettings) (model.AppSettings, error) {
	payload.OpenAIBaseURL = strings.TrimRight(strings.TrimSpace(payload.OpenAIBaseURL), "/")
	if payload.OpenAIBaseURL == "" {
		payload.OpenAIBaseURL = model.DefaultOpenAIBaseURL
	}
	u, err := url.Parse(payload.OpenAIBaseURL)
	if err != nil || u.Host == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" || (u.Scheme != "https" && !activity.LocalEndpoint(payload.OpenAIBaseURL)) {
		return model.AppSettings{}, openAITestValidationError{zh: "外部模型地址须使用 HTTPS，且不能包含用户名、密码或查询参数。", en: "External model endpoints require HTTPS without credentials or query parameters."}
	}
	previous := current.OpenAIBaseURL
	if strings.TrimSpace(previous) == "" {
		previous = model.DefaultOpenAIBaseURL
	}
	old, _ := url.Parse(previous)
	incoming := strings.TrimSpace(payload.OpenAIAPIKey)
	usingSaved := incoming == "" || incoming == redactSecret(current.OpenAIAPIKey)
	if current.OpenAIAPIKey != "" && usingSaved && (old == nil || !strings.EqualFold(old.Scheme+"://"+old.Host, u.Scheme+"://"+u.Host)) {
		return model.AppSettings{}, openAITestValidationError{zh: "更换模型服务商后，请重新输入该服务商的 API Key。", en: "After changing providers, enter that provider's API key again."}
	}
	payload.OpenAIAPIKey = preservedSecret(payload.OpenAIAPIKey, current.OpenAIAPIKey)
	return payload, nil
}

func (r *Router) handleOpenAIModels(w http.ResponseWriter, req *http.Request) {
	var payload model.AppSettings
	if httpx.DecodeJSON(req, &payload) != nil {
		httpx.Error(w, 400, "invalid_settings")
		return
	}
	current, err := r.store.Settings.Get(req.Context())
	if err != nil {
		httpx.Error(w, 500, "settings_error")
		return
	}
	settings, err := prepareModelEndpoint(payload, current)
	if err != nil {
		e := err.(openAITestValidationError)
		httpx.Error(w, 400, r.localized(req.Context(), e.zh, e.en))
		return
	}
	if settings.OpenAIAPIKey == "" && !activity.LocalEndpoint(settings.OpenAIBaseURL) {
		httpx.Error(w, 400, r.localized(req.Context(), "请先在本机填写 API Key。", "Enter the API key locally first."))
		return
	}
	client := openai.New(openai.Config{BaseURL: settings.OpenAIBaseURL, APIKey: settings.OpenAIAPIKey, Timeout: 20 * time.Second})
	items, err := client.ListModels(req.Context())
	if err != nil {
		httpx.Error(w, 502, r.localized(req.Context(), "无法获取模型列表，请检查地址和密钥，或手动填写平台提供的模型名称。", "Could not fetch models. Check the endpoint and key, or enter a model ID from your provider."))
		return
	}
	httpx.JSON(w, 200, map[string]any{"items": items})
}
