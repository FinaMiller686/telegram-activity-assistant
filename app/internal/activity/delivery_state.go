package activity

import (
	"strconv"
	"strings"
)

func deliveryNeedsReview(code string) bool {
	return code == "delivery_sending" || code == "delivery_unconfirmed"
}

func deliveryFailureCode(err error) string {
	code := err.Error()
	if code == "bot_request_invalid" || code == "bot_document_invalid" {
		return "bot_send_failed"
	}
	const prefix = "bot_api_status_"
	if strings.HasPrefix(code, prefix) {
		status, parseErr := strconv.Atoi(strings.TrimPrefix(code, prefix))
		// Request timeouts and 5xx responses can be ambiguous. Only an
		// explicit client-side rejection allows an unattended retry.
		if parseErr == nil && status >= 400 && status < 500 && status != 408 {
			return "bot_send_failed"
		}
	}
	return "delivery_unconfirmed"
}
