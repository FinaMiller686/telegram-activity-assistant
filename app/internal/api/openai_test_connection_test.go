package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/fr0der1c/tgtldr/app/internal/model"
	"github.com/fr0der1c/tgtldr/app/internal/openai"
	. "github.com/smartystreets/goconvey/convey"
)

func TestSafeModelTestError(t *testing.T) {
	err := &openai.APIError{StatusCode: 403, Message: "当前网络环境不支持", Response: "fixture-secret"}
	result := safeModelTestError(err)
	if result.zh != "模型平台拒绝当前网络环境。请按平台要求连接获准的校园网络或学校官方 VPN 后重试。" {
		t.Fatalf("unexpected classification: %s", result.zh)
	}
	err.Message = "echoed fixture-secret"
	if result = safeModelTestError(err); strings.Contains(result.zh, "fixture-secret") {
		t.Fatal("provider error leaked")
	}
}

func TestPrepareOpenAITestSettings(t *testing.T) {
	current := model.AppSettings{
		OpenAIAPIKey: "saved-key",
	}

	Convey("空 Base URL 会使用默认值", t, func() {
		settings, err := prepareOpenAITestSettings(model.AppSettings{
			OpenAIAPIKey:      "test-key",
			OpenAIModel:       "gpt-test",
			OpenAITemperature: 0.2,
		}, current)

		So(err, ShouldBeNil)
		So(settings.OpenAIBaseURL, ShouldEqual, model.DefaultOpenAIBaseURL)
	})

	Convey("空 API Key 会保留已保存密钥", t, func() {
		settings, err := prepareOpenAITestSettings(model.AppSettings{
			OpenAIModel:       "gpt-test",
			OpenAITemperature: 0.2,
		}, current)

		So(err, ShouldBeNil)
		So(settings.OpenAIAPIKey, ShouldEqual, "saved-key")
	})

	Convey("缺少模型会报错", t, func() {
		_, err := prepareOpenAITestSettings(model.AppSettings{
			OpenAIAPIKey:      "test-key",
			OpenAITemperature: 0.2,
		}, current)

		So(err, ShouldNotBeNil)
		So(err.Error(), ShouldContainSubstring, "Model")
	})

	Convey("空调用方式会默认使用流式", t, func() {
		settings, err := prepareOpenAITestSettings(model.AppSettings{
			OpenAIAPIKey:      "test-key",
			OpenAIModel:       "gpt-test",
			OpenAITemperature: 0.2,
		}, current)

		So(err, ShouldBeNil)
		So(settings.OpenAIRequestMode, ShouldEqual, model.OpenAIRequestModeStream)
	})

	Convey("非法调用方式会报错", t, func() {
		_, err := prepareOpenAITestSettings(model.AppSettings{
			OpenAIAPIKey:      "test-key",
			OpenAIModel:       "gpt-test",
			OpenAITemperature: 0.2,
			OpenAIRequestMode: "invalid",
		}, current)

		So(err, ShouldNotBeNil)
		So(err.Error(), ShouldContainSubstring, "调用方式")
	})
}

func TestOpenAITestRouteRegistration(t *testing.T) {
	Convey("OpenAI 测试路由可以和设置路由一起注册", t, func() {
		So(func() {
			_ = (&Router{}).Handler()
		}, ShouldNotPanic)
	})
}

func TestRunOpenAITest(t *testing.T) {
	Convey("测试连接会按非流式模式发送低成本 chat completion 请求", t, func() {
		var payload map[string]any
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"model":"gpt-test","choices":[{"message":{"role":"assistant","content":"OK"}}]}`))
		}))
		defer server.Close()

		result, err := runOpenAITest(context.Background(), model.AppSettings{
			OpenAIBaseURL:     server.URL,
			OpenAIAPIKey:      "test-key",
			OpenAIModel:       "gpt-test",
			OpenAITemperature: 0.2,
			OpenAIRequestMode: model.OpenAIRequestModeNonStream,
		})

		So(err, ShouldBeNil)
		So(result.OK, ShouldBeTrue)
		So(result.Model, ShouldEqual, "gpt-test")
		So(payload["max_completion_tokens"], ShouldEqual, float64(256))
		_, hasStreamField := payload["stream"]
		So(hasStreamField, ShouldBeFalse)
	})

	Convey("测试连接会按流式模式发送请求", t, func() {
		var payload map[string]any
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			w.Header().Set("Content-Type", "text/event-stream")
			_, _ = w.Write([]byte(`data: {"model":"gpt-test","choices":[{"delta":{"content":"OK"}}]}` + "\n\n"))
			_, _ = w.Write([]byte("data: [DONE]\n\n"))
		}))
		defer server.Close()

		result, err := runOpenAITest(context.Background(), model.AppSettings{
			OpenAIBaseURL:     server.URL,
			OpenAIAPIKey:      "test-key",
			OpenAIModel:       "gpt-test",
			OpenAITemperature: 0.2,
			OpenAIRequestMode: model.OpenAIRequestModeStream,
		})

		So(err, ShouldBeNil)
		So(result.OK, ShouldBeTrue)
		So(result.Model, ShouldEqual, "gpt-test")
		So(payload["stream"], ShouldEqual, true)
	})

	Convey("上游错误会原样返回给调用方", t, func() {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			http.Error(w, "bad gateway", http.StatusBadGateway)
		}))
		defer server.Close()

		_, err := runOpenAITest(context.Background(), model.AppSettings{
			OpenAIBaseURL:     server.URL,
			OpenAIAPIKey:      "test-key",
			OpenAIModel:       "gpt-test",
			OpenAITemperature: 0.2,
			OpenAIRequestMode: model.OpenAIRequestModeNonStream,
		})

		So(err, ShouldNotBeNil)
		So(err.Error(), ShouldContainSubstring, "openai status 502")
	})
}
