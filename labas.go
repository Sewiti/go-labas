package labas

import (
	"bytes"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"regexp"
	"strings"
)

const (
	baseURL    = "https://mano.labas.lt"
	loginRoute = "/prisijungimo_patikrinimas"

	// How many times client attempts to send SMS, trying to relogin after an
	// unsuccessful attempt.
	attempts = 2
)

type client struct {
	user  string       // Username (phone number).
	pass  string       // User's password.
	token string       // A token used for sending SMS.
	scml  string       // A cookie used for authentication.
	http  *http.Client // A HTTP client used for performing requests.
}

// Client is a simple Labas client that can send SMS messages using Labas web
// services.
type Client interface {
	// SetHTTPClient sets a new HTTP client.
	SetHTTPClient(http *http.Client)

	// SendSMS sends a message to a recipient, returns nil if successful.
	SendSMS(rec string, msg string) error
}

var ErrUnableToSendSMS = errors.New("labas: unable to send sms")
var ErrUnableToGetSCML = errors.New("labas: unable to get scml")
var ErrUnableToGetToken = errors.New("labas: unable to get token")

// NewClient creates a new Labas client with a given username (phone number) and
// password, that uses http.DefaultClient.
func NewClient(user string, pass string) Client {
	return &client{
		user: user,
		pass: pass,
		http: http.DefaultClient,
	}
}

// SetHTTPClient sets a new HTTP client.
func (cl *client) SetHTTPClient(http *http.Client) {
	cl.http = http
}

// SendSMS sends a message to a recipient, returns nil if successful.
//
// Tries multiple times (if unsuccessful), performing relogin after each
// unsuccessful attempt.
func (cl *client) SendSMS(rec string, msg string) error {
	// Login if first time
	if cl.token == "" {
		if err := cl.login(); err != nil {
			return err
		}
	}

	for i := 0; i < attempts; i++ {
		sent, err := cl.sendSMS(rec, msg)
		if err != nil {
			return err
		}

		if sent {
			// Exit if successful
			return nil
		}

		if i < attempts-1 {
			// Try relogging in, maybe tokens have expired
			if err := cl.login(); err != nil {
				return err
			}
		}
	}

	return ErrUnableToSendSMS
}

// sendSMS performs the requests to send sms, returns sent true if successful.
//
// If error occurs, sent is always false.
func (cl *client) sendSMS(rec string, msg string) (sent bool, err error) {
	data := url.Values{
		"sms_submit[recipientNumber]": {rec},
		"sms_submit[textMessage]":     {msg},
		"sms_submit[_token]":          {cl.token},
	}.Encode()

	req, err := http.NewRequest(http.MethodPost, baseURL,
		strings.NewReader(data))
	if err != nil {
		return
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Cookie", fmt.Sprintf("scml=%s", cl.scml))

	res, err := cl.http.Do(req)
	if err != nil {
		return
	}
	defer res.Body.Close()

	body, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return
	}

	if bytes.Contains(body, []byte("SMS išsiųsta")) {
		sent = true
	}

	return
}

// login into mano.labas.lt, saving scml cookie and token needed for SMS
// sending.
func (cl *client) login() error {
	if err := cl.renewSCML(); err != nil {
		return err
	}

	if err := cl.renewToken(); err != nil {
		return err
	}

	return nil
}

// renewSCML performs an actual login and gets a scml cookie that is used for
// authorization purposes by Labas.
func (cl *client) renewSCML() error {
	data := url.Values{
		"_username": {cl.user},
		"_password": {cl.pass},
	}.Encode()

	req, err := http.NewRequest(http.MethodPost, baseURL+loginRoute,
		strings.NewReader(data))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Cookie", "scml=; TS011605d9=0")

	res, err := cl.http.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()

	// Get scml cookie
	for _, cookie := range res.Cookies() {
		if cookie.Name == "scml" {
			cl.scml = cookie.Value
			return nil
		}
	}

	return ErrUnableToGetSCML
}

var tokenRegex = regexp.MustCompile(`<input.+?name=\"sms_submit\[_token\]\".*?value=\"(.*?)\".*?\/>`)

// renewToken gets a token used for sending SMS messages.
//
// Requires valid scml cookie to be successful.
func (cl *client) renewToken() error {
	// Get homepage
	req, err := http.NewRequest(http.MethodGet, baseURL, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Cookie", fmt.Sprintf("scml=%s", cl.scml))

	res, err := cl.http.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()

	body, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return err
	}

	// Extract SMS token from homepage
	match := tokenRegex.FindSubmatch(body)
	if match == nil || len(match) < 2 {
		return ErrUnableToGetToken
	}

	cl.token = string(match[1])

	return nil
}
