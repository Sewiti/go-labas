package labas

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"strings"
	"sync"

	"golang.org/x/net/html"
	"golang.org/x/net/html/atom"
)

var (
	ErrSendSMS = errors.New("send sms")
	ErrLogin   = errors.New("login")
)

type Client interface {
	// SendSMS sends a message to a recipient, returns nil if successful.
	//
	// Tries multiple times (if unsuccessful), performing relogin after each
	// unsuccessful attempt.
	SendSMS(recipient, message string) error

	// SendSMSContext sends a message to a recipient, returns nil if successful.
	//
	// Tries multiple times (if unsuccessful), performing relogin after each
	// unsuccessful attempt.
	SendSMSContext(ctx context.Context, recipient, message string) error
}

type client struct {
	username string
	password string

	attempts   int
	baseURL    string
	loginRoute string

	http  *http.Client
	token string

	mx sync.Mutex
}

func NewClient(username, password string) Client {
	jar, _ := cookiejar.New(nil)
	return &client{
		username: username,
		password: password,

		attempts:   2,
		baseURL:    "https://mano.labas.lt",
		loginRoute: "/prisijungimo_patikrinimas",

		http: &http.Client{Jar: jar},
	}
}

func (cl *client) SendSMS(recipient, message string) error {
	return cl.SendSMSContext(context.Background(), recipient, message)
}

func (cl *client) SendSMSContext(ctx context.Context, recipient, message string) error {
	cl.mx.Lock()
	defer cl.mx.Unlock()

	if cl.token == "" {
		if err := cl.login(ctx); err != nil {
			return fmt.Errorf("labas: %w", err)
		}
	}

	for i := 0; i < cl.attempts; i++ {
		sent, err := cl.sendSMS(ctx, recipient, message)
		if err != nil {
			return fmt.Errorf("labas: %w", err)
		}
		if sent {
			return nil
		}

		if i < cl.attempts-1 {
			if err := cl.login(ctx); err != nil {
				return fmt.Errorf("labas: %w", err)
			}
		}
	}
	return fmt.Errorf("labas: %w: ", ErrSendSMS)
}

// sendSMS performs the requests to send sms, returns sent true if successful.
func (cl *client) sendSMS(ctx context.Context, recipient, message string) (bool, error) {
	data := url.Values{
		"sms_submit[recipientNumber]": {recipient},
		"sms_submit[textMessage]":     {message},
		"sms_submit[_token]":          {cl.token},
	}.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, cl.baseURL, strings.NewReader(data))
	if err != nil {
		return false, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	res, err := cl.http.Do(req)
	if err != nil {
		return false, err
	}
	defer res.Body.Close()

	body, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return false, nil
	}
	return bytes.Contains(body, []byte("SMS išsiųsta")), nil
}

func (cl *client) login(ctx context.Context) error {
	if err := cl.login1(ctx); err != nil {
		return fmt.Errorf("%w: %v", ErrLogin, err)
	}
	if err := cl.login2(ctx); err != nil {
		return fmt.Errorf("%w: %v", ErrLogin, err)
	}
	return nil
}

func (cl *client) login1(ctx context.Context) error {
	data := url.Values{
		"_username": {cl.username},
		"_password": {cl.password},
	}.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, cl.baseURL+cl.loginRoute, strings.NewReader(data))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	res, err := cl.http.Do(req)
	if err != nil {
		return err
	}
	err = res.Body.Close()
	return err
}

func (cl *client) login2(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, cl.baseURL, nil)
	if err != nil {
		return err
	}

	res, err := cl.http.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()

	n, err := html.Parse(res.Body)
	if err != nil {
		return err
	}

	token, err := getSMSToken(n)
	if err != nil {
		return err
	}
	cl.token = token
	return nil
}

func getSMSToken(n *html.Node) (string, error) {
	// Looking for:
	//  <input
	//      type="hidden"
	//      id="sms_submit__token"
	//      name="sms_submit[_token]"
	//      value="pXoZYkVsiTmj0KFuILwx4EBFbtCY2PK0JHqWinrXuO4"
	//      class="form-control input-material" />
	n = traverseHtmlNode(n, func(n *html.Node) bool {
		if n.DataAtom != atom.Input {
			return false
		}
		for _, attr := range n.Attr {
			if attr.Key == "name" {
				return attr.Val == "sms_submit[_token]"
			}
		}
		return false
	})
	if n == nil {
		return "", errors.New("input sms_submit[_token]: not found")
	}

	for _, attr := range n.Attr {
		if attr.Key == "value" {
			return attr.Val, nil
		}
	}
	return "", errors.New("input sms_submit[_token]: value attribute not found")
}

func traverseHtmlNode(n *html.Node, fn func(*html.Node) bool) *html.Node {
	var q []*html.Node
	pop := func() *html.Node {
		i := len(q) - 1
		if i < 0 {
			return nil
		}
		n := q[i]
		q = q[:i]
		return n
	}
	for ; n != nil; n = pop() {
		if fn(n) {
			return n
		}
		if n.NextSibling != nil {
			q = append(q, n.NextSibling)
		}
		if n.FirstChild != nil {
			q = append(q, n.FirstChild)
		}
	}
	return nil
}
