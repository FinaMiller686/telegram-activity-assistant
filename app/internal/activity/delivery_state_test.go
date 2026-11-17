package activity

import (
	"errors"
	"testing"
)

func TestDeliveryFailureCode(t *testing.T) {
	for _, test := range []struct {
		code string
		want string
	}{
		{"bot_network_failed", "delivery_unconfirmed"},
		{"bot_response_invalid", "delivery_unconfirmed"},
		{"unexpected EOF", "delivery_unconfirmed"},
		{"context canceled", "delivery_unconfirmed"},
		{"bot_api_status_500", "delivery_unconfirmed"},
		{"bot_api_status_502", "delivery_unconfirmed"},
		{"bot_api_status_408", "delivery_unconfirmed"},
		{"bot_api_status_400", "bot_send_failed"},
		{"bot_api_status_401", "bot_send_failed"},
		{"bot_api_status_403", "bot_send_failed"},
		{"bot_api_status_429", "bot_send_failed"},
		{"bot_request_invalid", "bot_send_failed"},
		{"bot_document_invalid", "bot_send_failed"},
	} {
		t.Run(test.code, func(t *testing.T) {
			if got := deliveryFailureCode(errors.New(test.code)); got != test.want {
				t.Fatalf("got %q, want %q", got, test.want)
			}
		})
	}
}

func TestDeliveryNeedsReview(t *testing.T) {
	for _, test := range []struct {
		code string
		want bool
	}{
		{"delivery_sending", true},
		{"delivery_unconfirmed", true},
		{"bot_send_failed", false},
		{"", false},
	} {
		if got := deliveryNeedsReview(test.code); got != test.want {
			t.Errorf("%q: got %v, want %v", test.code, got, test.want)
		}
	}
}
