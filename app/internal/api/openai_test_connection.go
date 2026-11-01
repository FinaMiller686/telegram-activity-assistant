package api

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/fr0der1c/tgtldr/app/internal/activity"
	"github.com/fr0der1c/tgtldr/app/internal/httpx"
	"github.com/fr0der1c/tgtldr/app/internal/model"
	"github.com/fr0der1c/tgtldr/app/internal/openai"
)

type openAITestResult struct {
	OK    bool   `json:"ok"`
	Model string `json:"model"`
}

type openAITestValidationError struct {
	zh string
	en string
}

func (e openAITestValidationError) Error() string {
	return e.zh
}

func (r *Router) handleOpenAITest(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodPost {
		httpx.Error(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	var payload model.AppSettings
	if err := httpx.DecodeJSON(req, &payload); err != nil {
		httpx.Error(w, http.StatusBadRequest, err.Error())
		return
	}
	current, err := r.store.Settings.Get(req.Context())
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, err.Error())
		return
	}

	settings, err := prepareOpenAITestSettings(payload, current)
	if err != nil {
		var validationErr openAITestValidationError
		if errors.As(err, &validationErr) {
			httpx.Error(w, http.StatusBadRequest, r.localized(req.Context(), validationErr.zh, validationErr.en))
			return
		}
		httpx.Error(w, http.StatusBadRequest, err.Error())
		return
	}

	result, err := runOpenAITest(req.Context(), settings)
	if err != nil {
		detail := safeModelTestError(err)
		httpx.Error(w, http.StatusBadGateway, r.localized(req.Context(), detail.zh, detail.en))
		return
	}
	httpx.JSON(w, http.StatusOK, result)
}

// Never reflect provider response bodies: they may echo credentials or identifiers.
func safeModelTestError(err error) openAITestValidationError {
	var upstream *openai.APIError
	if errors.As(err, &upstream) {
		if strings.Contains(upstream.Message, "当前网络环境不支持") {
			return openAITestValidationError{zh: "模型平台拒绝当前网络环境。请按平台要求连接获准的校园网络或学校官方 VPN 后重试。", en: "The model provider does not support this network. Use a permitted campus network or official university VPN as required by the provider, then retry."}
		}
		switch upstream.StatusCode {
		case 401:
			return openAITestValidationError{zh: "模型平台未接受密钥，请检查密钥是否正确、有效。", en: "The provider did not accept the key. Check that it is correct and active."}
		case 403:
			return openAITestValidationError{zh: "模型平台拒绝请求（403）。请检查账号、模型权限和平台允许的网络。", en: "The provider rejected the request (403). Check account permissions, model access, and permitted networks."}
		case 404:
			return openAITestValidationError{zh: "模型或接口地址不存在（404），请核对平台提供的准确名称和地址。", en: "Model or endpoint not found (404). Check the exact model ID and endpoint."}
		case 429:
			return openAITestValidationError{zh: "模型平台限流或额度不足（429），请检查平台用量后重试。", en: "Rate limit or quota exceeded (429). Check provider usage before retrying."}
		}
		if upstream.StatusCode > 0 {
			return openAITestValidationError{zh: fmt.Sprintf("模型平台返回错误（HTTP %d），请检查平台状态和接口配置。", upstream.StatusCode), en: fmt.Sprintf("The provider returned HTTP %d. Check provider status and API settings.", upstream.StatusCode)}
		}
	}
	if errors.Is(err, openai.ErrEmptyResponse) {
		return openAITestValidationError{zh: "模型返回空正文，请检查模型是否需要更高的输出限制或关闭推理模式。", en: "The model returned no final text. Check its output limit or reasoning mode."}
	}
	return openAITestValidationError{zh: "模型连接测试失败，请检查模型名称、密钥、额度和接口地址。", en: "Model connection failed. Check the model ID, key, quota, and endpoint."}
}

func prepareOpenAITestSettings(payload, current model.AppSettings) (model.AppSettings, error) {
	var err error
	payload, err = prepareModelEndpoint(payload, current)
	if err != nil {
		return model.AppSettings{}, err
	}

	if strings.TrimSpace(payload.OpenAIBaseURL) == "" {
		payload.OpenAIBaseURL = model.DefaultOpenAIBaseURL
	}
	if strings.TrimSpace(payload.OpenAIAPIKey) == "" && !activity.LocalEndpoint(payload.OpenAIBaseURL) {
		return model.AppSettings{}, openAITestValidationError{
			zh: "请填写 OpenAI API Key。",
			en: "Enter OpenAI API Key.",
		}
	}
	if strings.TrimSpace(payload.OpenAIModel) == "" {
		return model.AppSettings{}, openAITestValidationError{
			zh: "请填写 Model。",
			en: "Enter Model.",
		}
	}
	if payload.OpenAITemperature < 0 || payload.OpenAITemperature > 2 {
		return model.AppSettings{}, openAITestValidationError{
			zh: "Temperature 必须在 0.0 到 2.0 之间。",
			en: "Temperature must be between 0.0 and 2.0.",
		}
	}
	if payload.OpenAIRequestMode == "" {
		payload.OpenAIRequestMode = model.OpenAIRequestModeStream
	}
	if payload.OpenAIRequestMode != model.OpenAIRequestModeStream && payload.OpenAIRequestMode != model.OpenAIRequestModeNonStream {
		return model.AppSettings{}, openAITestValidationError{
			zh: "调用方式必须是 stream 或 non_stream。",
			en: "Request mode must be stream or non_stream.",
		}
	}
	return payload, nil
}

func runOpenAITest(ctx context.Context, settings model.AppSettings) (openAITestResult, error) {
	client := openai.New(openai.Config{
		BaseURL: settings.OpenAIBaseURL,
		APIKey:  settings.OpenAIAPIKey,
		Model:   settings.OpenAIModel,
		Stream:  settings.OpenAIRequestMode != model.OpenAIRequestModeNonStream,
	})
	resp, err := client.Chat(ctx, openai.ChatRequest{
		SystemPrompt:    "Reply with exactly OK.",
		UserPrompt:      "Connection test. Reply with exactly OK.",
		Temperature:     settings.OpenAITemperature,
		MaxOutput:       256,
		ReasoningEffort: "none",
	})
	if err != nil {
		return openAITestResult{}, err
	}
	return openAITestResult{
		OK:    true,
		Model: strings.TrimSpace(resp.Model),
	}, nil
}
