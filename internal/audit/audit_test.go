package audit

import "testing"

func TestEventValidationAndSafeMetadata(t *testing.T) {
	t.Parallel()
	for _, actor := range []Actor{ActorSystem, ActorUser, ActorPlugin} {
		if err := (Event{Name: "test.recorded", Actor: actor, Subject: "subject"}).Validate(); err != nil {
			t.Errorf("actor %q rejected: %v", actor, err)
		}
	}
	if err := (Event{Name: "test.recorded", Actor: "unknown", Subject: "subject"}).Validate(); err == nil {
		t.Fatal("invalid actor accepted")
	}

	metadata := SafeMetadata(map[string]string{
		"provider_token": "secret-value",
		"answer":         "student answer",
		"scope":          "project",
	})
	if metadata["provider_token"] != redacted || metadata["answer"] != omitted || metadata["scope"] != "project" {
		t.Fatalf("SafeMetadata() = %#v", metadata)
	}
}
