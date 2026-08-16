package cli

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os/exec"
	"runtime"
	"time"

	"github.com/urfave/cli/v3"
)

const loginTimeout = 5 * time.Minute

func CmdLogin() *cli.Command {
	return &cli.Command{
		Name:        "login",
		Description: "login to remote jittrippin instance",
		Flags: []cli.Flag{
			&cli.StringFlag{Name: "provider", Value: "github", Usage: "OAuth provider"},
			&cli.StringFlag{Name: "daemon", Usage: "daemon base URL (default: from config)"},
			&cli.BoolFlag{Name: "no-open", Usage: "print the URL instead of opening a browser"},
		},
		Action: handleLogin,
	}
}

func handleLogin(ctx context.Context, c *cli.Command) error {
	base, err := daemonURL(c)
	if err != nil {
		return err
	}
	client := newDaemonClient(base)
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return err
	}
	defer ln.Close()

	redirect := fmt.Sprintf("http://127.0.0.1:%d/callback", ln.Addr().(*net.TCPAddr).Port)
	url, err := client.Begin(c.String("provider"), redirect)
	if err != nil {
		return err
	}

	fmt.Printf("Open this URL in your browser:\n\n	%s\n\n", url)
	if !c.Bool("no-open") {
		_ = openBrowser(url)
	}

	codeCh := make(chan string, 1)
	errCh := make(chan error, 1)
	srv := &http.Server{Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		code := r.URL.Query().Get("auth_code")
		if code == "" {
			http.Error(w, "missing auth_code", http.StatusBadRequest)
			return
		}
		select {
		case codeCh <- code:
		default:
		}
		// todo: change this to html perhaps, or not...
		w.Header().Set("Content-Type", "text/plain")
		w.Write([]byte("You can close this tab and return to the terminal."))
	})}
	go func() { errCh <- srv.Serve(ln) }()

	var authCode string
	select {
	case authCode = <-codeCh:
	case err := <-errCh:
		return err
	case <-time.After(loginTimeout):
		return errors.New("timed out: login not completed in time")
	case <-ctx.Done():
		return ctx.Err()
	}
	_ = srv.Close()
	token, err := client.Exchange(authCode)
	if err != nil {
		return err
	}
	if err := saveSession(
		&Session{Token: token, Provider: c.String("provider")},
	); err != nil {
		return err
	}
	fmt.Println("	Logged in successfully.\n ")
	return nil
}

func openBrowser(url string) error {
	var cmd string
	var args []string
	switch runtime.GOOS {
	case "darwin":
		cmd, args = "open", []string{url}
	case "windows":
		cmd, args = "cmd", []string{"/c", "start", url}
	default:
		cmd, args = "xdg-open", []string{url}
	}
	return exec.Command(cmd, args...).Start()
}
