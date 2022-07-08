package labas

import (
	"testing"
)

func TestSendSMS(t *testing.T) {
	const (
		user = "USERNAME"
		pass = "PASSWORD"
		rec  = "RECIPIENT"
	)

	cl := NewClient(user, pass)

	if err := cl.SendSMS(rec, "Test"); err != nil {
		t.Error(err)
		t.FailNow()
	}
	if err := cl.SendSMS(rec, "Test 2"); err != nil {
		t.Error(err)
		t.FailNow()
	}
}
