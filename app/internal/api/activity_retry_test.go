package api

import (
	"net/http/httptest"
	"strings"
	"testing"
)

func TestActivityDeliveryRetryRequiresExplicitConfirmation(t *testing.T) {
	for _, body := range []string{
		`{"deliveryKey":"probe:unconfirmed"}`,
		`{"deliveryKey":"probe:unconfirmed","confirmUnreceived":false}`,
		`{"deliveryKey":"` + strings.Repeat("x", 257) + `","confirmUnreceived":true}`,
		`{"deliveryKey":`,
	} {
		req := httptest.NewRequest("POST", "/api/activity/retry", strings.NewReader(body))
		out := httptest.NewRecorder()
		// A missing store makes any accidental mutation fail this test.
		(&Router{}).handleActivityRetry(out, req)
		if out.Code != 400 {
			t.Fatalf("unconfirmed retry returned %d", out.Code)
		}
	}
}
