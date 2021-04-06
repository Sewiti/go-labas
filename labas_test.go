package labas

import (
	"os"
	"testing"

	"github.com/joho/godotenv"
)

func TestSendSMS(t *testing.T) {
	err := godotenv.Load()
	if err != nil {
		t.Error(err)
	}

	user := os.Getenv("LABAS_USER")
	pass := os.Getenv("LABAS_PASSWORD")
	rec := os.Getenv("LABAS_RECIPIENT")

	cl := NewClient(user, pass)

	if err := cl.SendSMS(rec, "Test"); err != nil {
		t.Error(err)
	}
}
