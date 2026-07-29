package github

import "testing"

func TestUpsertComment_Disabled(t *testing.T) {
	if err := UpsertComment(NewDisabledClient(), "m", "body"); err != nil {
		t.Errorf("disabled client should no-op: %v", err)
	}
}

func TestDeleteComments_Disabled(t *testing.T) {
	if err := DeleteComments(NewDisabledClient(), "m"); err != nil {
		t.Errorf("disabled client should no-op: %v", err)
	}
}
