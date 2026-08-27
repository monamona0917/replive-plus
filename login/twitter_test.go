package login

import (
	"replive/model"
	"strings"
	"testing"

	"google.golang.org/protobuf/proto"
)

func TestParseTwitterCallback(t *testing.T) {
	token, verifier, err := parseTwitterCallback("replive-user-auth://user-auth?oauth_token=tok&oauth_verifier=ver")
	if err != nil {
		t.Fatalf("parseTwitterCallback() error = %v", err)
	}
	if token != "tok" {
		t.Fatalf("oauth token = %q, want tok", token)
	}
	if verifier != "ver" {
		t.Fatalf("oauth verifier = %q, want ver", verifier)
	}
}

func TestParseTwitterCallbackRejectsUnexpectedURL(t *testing.T) {
	_, _, err := parseTwitterCallback("https://example.com/?oauth_token=tok&oauth_verifier=ver")
	if err == nil {
		t.Fatal("parseTwitterCallback() error = nil, want error")
	}
}

func TestRedactURLMasksTwitterSecrets(t *testing.T) {
	got := redactURL("replive-user-auth://user-auth?oauth_token=abcdefghijklmnopqrstuvwxyz&oauth_verifier=1234567890abcdefghijklmnopqrstuvwxyz")
	if got == "replive-user-auth://user-auth?oauth_token=abcdefghijklmnopqrstuvwxyz&oauth_verifier=1234567890abcdefghijklmnopqrstuvwxyz" {
		t.Fatal("redactURL did not mask Twitter secrets")
	}
	if containsAny(got, "abcdefghijklmnopqrstuvwxyz", "1234567890abcdefghijklmnopqrstuvwxyz") {
		t.Fatalf("redactURL leaked secret: %s", got)
	}
}

func TestMarshalTwitterSNSLoginURLRequestMatchesCaptureShape(t *testing.T) {
	challenge := "H_uVWqCiOQlMXmoQmbWvZTu_wiPp9b5PauuJyxXxZwc"
	body, err := marshalTwitterSNSLoginURLRequest(&model.GetSNSLoginURLRequest{
		IdProvider:    model.IdProvider_ID_PROVIDER_TWITTER,
		CodeChallenge: challenge,
	})
	if err != nil {
		t.Fatalf("marshalTwitterSNSLoginURLRequest() error = %v", err)
	}
	if len(body) != 49 {
		t.Fatalf("request body length = %d, want 49", len(body))
	}
	if body[len(body)-2] != byte(4<<3|0) || body[len(body)-1] != 1 {
		t.Fatalf("request body does not end with field 4=true: %v", body[len(body)-2:])
	}

	var decoded model.GetSNSLoginURLRequest
	if err := proto.Unmarshal(body, &decoded); err != nil {
		t.Fatalf("proto.Unmarshal() error = %v", err)
	}
	if decoded.GetIdProvider() != model.IdProvider_ID_PROVIDER_TWITTER || decoded.GetState() != "" || decoded.GetCodeChallenge() != challenge {
		t.Fatalf("decoded request = %+v", decoded)
	}
}

func containsAny(s string, needles ...string) bool {
	for _, needle := range needles {
		if len(needle) > 0 && strings.Contains(s, needle) {
			return true
		}
	}
	return false
}
