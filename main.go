package main

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"regexp"
	"syscall"
	"time"

	"github.com/go-playground/validator/v10"
	expect "github.com/google/goexpect"
)

const (
	ONAMAE_SERVER = "ddnsclient.onamae.com:65010"
)

type Input struct {
	Username string `validate:"required"`
	Password string `validate:"required"`

	Hostname string `validate:"required"`
	Domain   string `validate:"required"`
	IP4      string `validate:"required"`

	Daemon   bool   `validate:"required"`
	Interval string `validate:"omitempty"`
}

var input Input

func init() {
	flag.StringVar(&input.Username, "u", "", "Username onamae.com.env:$ONAMAE_USERNAME")
	flag.StringVar(&input.Password, "p", "", "Password onamae.com env:$ONAMAE_PASSWORD")
	flag.StringVar(&input.Hostname, "h", "", "Hostname. ex. www")
	flag.StringVar(&input.Domain, "d", "", "Domain. ex. example.com")
	flag.StringVar(&input.IP4, "i", "", "IP address. If empty, will get it automatically using `https://httpbin.org/ip`")
	flag.BoolVar(&input.Daemon, "daemon", false, "Launch as daemon")
	flag.StringVar(&input.Interval, "interval", "5m", "Update interval. Enable only for daemon mode")
	flag.Parse()
}

func main() {
	ctx := context.Background()
	if !input.Daemon {
		err := execute(ctx, input)
		if err != nil {
			log.Fatal(err)
		}
		return
	}

	interval, err := time.ParseDuration(input.Interval)
	if err != nil {
		log.Fatal(err)
		return
	}

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGTERM, syscall.SIGINT)
	doneCh := make(chan struct{})
	go func() {
		defer cancel()
		<-sigCh
	}()
	go func() {
		defer close(doneCh)
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
			case <-ctx.Done():
				return
			}
			if err := execute(ctx, input); err != nil {
				log.Print(err)
			}
		}
	}()
	<-doneCh
}

func execute(ctx context.Context, input Input) error {
	if input.Username == "" {
		input.Username = os.Getenv("ONAMAE_USERNAME")
	}
	if input.Password == "" {
		input.Password = os.Getenv("ONAMAE_PASSWORD")
	}
	if input.IP4 == "" {
		log.Println("get automatically")
		resp, err := http.Get("https://httpbin.org/ip")
		if err != nil {
			return err
		}
		defer resp.Body.Close()
		obj := map[string]interface{}{}
		if err := json.NewDecoder(resp.Body).Decode(&obj); err != nil {
			return err
		}
		input.IP4 = obj["origin"].(string)

	}
	v := validator.New()
	if err := v.Struct(input); err != nil {
		return err
	}

	client, err := tls.Dial("tcp", ONAMAE_SERVER, nil)
	if err != nil {
		return err
	}
	resCh := make(chan error)
	exp, _, err := expect.SpawnGeneric(&expect.GenOptions{
		In:  client,
		Out: client,
		Wait: func() error {
			return <-resCh
		},
		Close: func() error {
			close(resCh)
			return client.Close()
		},
		Check: func() bool { return true },
	}, time.Second, expect.Verbose(true))
	if err != nil {
		return err
	}

	login(exp, input.Username, input.Password)
	modify(exp, input.Hostname, input.Domain, input.IP4)
	logout(exp)
	return exp.Close()
}

func login(exp *expect.GExpect, username, password string) {
	exp.Send(fmt.Sprintf(`LOGIN
USERID:%s
PASSWORD:%s
.
`, username, password))
	exp.Expect(regexp.MustCompile(`\d{3}`), time.Second*10)
}

func logout(exp *expect.GExpect) {
	exp.Send(`LOGOUT
.
`)
	exp.Expect(regexp.MustCompile(`\d{3}`), time.Second*10)
}

func modify(exp *expect.GExpect, hostname, domainname, ipv4 string) {
	exp.Send(fmt.Sprintf(`MODIP
HOSTNAME:%s
DOMNAME:%s
IPV4:%s
.
`, hostname, domainname, ipv4))
	exp.Expect(regexp.MustCompile(`\d{3}`), time.Second*10)
}
