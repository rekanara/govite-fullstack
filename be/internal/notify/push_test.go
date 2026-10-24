package notify

import "testing"

func TestInvalidFCMTokenOnlyForDeadDevices(t *testing.T) {
	if !invalidFCMToken([]byte(`{"error":{"status":"NOT_FOUND","details":[{"errorCode":"UNREGISTERED"}]}}`)) {
		t.Fatal("UNREGISTERED must delete the token")
	}
	if invalidFCMToken([]byte(`{"error":{"status":"INVALID_ARGUMENT","message":"bad data payload"}}`)) {
		t.Fatal("a malformed payload must not delete a valid token")
	}
}
