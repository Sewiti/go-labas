package labas

import (
	"fmt"
	"os"
)

func ExampleClient_SendSMS() {
	const (
		user = "LABAS_USERNAME"
		pass = "LABAS_PASSWD"
		rec  = "RECIPIENT"
	)

	cl := NewClient(user, pass)

	if err := cl.SendSMS(rec, "Test"); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if err := cl.SendSMS(rec, "Test 2"); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
