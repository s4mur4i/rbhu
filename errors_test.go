package rbhu

import (
	"strings"
	"testing"
)

func TestParseAPIErrorBerlinEnvelope(t *testing.T) {
	body := []byte(`{"errorCode":"CONSENT_INVALID","errorDescription":"consent expired"}`)
	e := parseAPIError(403, "req-1", body)
	if e.StatusCode != 403 || e.RequestID != "req-1" {
		t.Fatalf("unexpected: %+v", e)
	}
	if e.ErrorCode != "CONSENT_INVALID" || e.ErrorDescription != "consent expired" {
		t.Fatalf("not parsed: %+v", e)
	}
	if !strings.Contains(e.Error(), "CONSENT_INVALID") {
		t.Errorf("Error() = %q", e.Error())
	}
}

func TestParseAPIErrorTPPMessages(t *testing.T) {
	body := []byte(`{"tppMessages":[{"category":"ERROR","code":"FORMAT_ERROR","text":"bad field"}]}`)
	e := parseAPIError(400, "", body)
	if len(e.TPPMessages) != 1 || e.TPPMessages[0].Code != "FORMAT_ERROR" {
		t.Fatalf("tppMessages not parsed: %+v", e)
	}
	if !strings.Contains(e.Error(), "FORMAT_ERROR") {
		t.Errorf("Error() = %q", e.Error())
	}
}

func TestParseAPIErrorRawBody(t *testing.T) {
	body := []byte("upstream timeout")
	e := parseAPIError(504, "", body)
	if !strings.Contains(e.Error(), "504") || !strings.Contains(e.Error(), "upstream timeout") {
		t.Errorf("Error() = %q", e.Error())
	}
}
