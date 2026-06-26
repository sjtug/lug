package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

const defaultAPI = "http://localhost:7001"

type client struct {
	base string
	http *http.Client
}

func main() {
	if len(os.Args) < 2 {
		usage(os.Stderr)
		os.Exit(2)
	}

	api := os.Getenv("LUG_API")
	if api == "" {
		api = defaultAPI
	}
	c := client{
		base: strings.TrimRight(api, "/"),
		http: &http.Client{Timeout: 15 * time.Second},
	}

	cmd := os.Args[1]
	args := os.Args[2:]
	var err error
	switch cmd {
	case "summary":
		err = c.summary(args)
	case "status":
		err = c.status(args)
	case "sync":
		err = c.sync(args)
	case "start":
		err = c.simple(args, http.MethodPost, "/lug/v1/admin/manager/start")
	case "stop":
		err = c.simple(args, http.MethodPost, "/lug/v1/admin/manager/stop")
	case "exit":
		err = c.simple(args, http.MethodDelete, "/lug/v1/admin/manager")
	case "help", "-h", "--help":
		usage(os.Stdout)
		return
	default:
		err = fmt.Errorf("unknown command %q", cmd)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "lugctl:", err)
		os.Exit(1)
	}
}

func usage(w io.Writer) {
	fmt.Fprint(w, `lugctl controls a running lug daemon over its HTTP API.

Usage:
  lugctl summary                 Print manager summary JSON
  lugctl status [--repo NAME]    Print full manager detail or one repo status
  lugctl sync --repo NAME        Trigger one repo sync in the live daemon
  lugctl start                   Resume manager polling
  lugctl stop                    Stop manager polling
  lugctl exit                    Gracefully stop the daemon

Environment:
  LUG_API   Daemon API base URL (default: http://localhost:7001)

Examples:
  docker compose exec lug lugctl sync --repo ubuntu
  docker compose exec lug lugctl status --repo ubuntu
`)
}

func (c client) summary(args []string) error {
	fs := newFlagSet("summary")
	api := fs.String("api", c.base, "daemon API base URL")
	if err := fs.Parse(args); err != nil {
		return err
	}
	c.base = strings.TrimRight(*api, "/")
	return c.doJSON(http.MethodGet, "/lug/v1/manager/summary", nil)
}

func (c client) status(args []string) error {
	fs := newFlagSet("status")
	api := fs.String("api", c.base, "daemon API base URL")
	repo := fs.String("repo", "", "repo name")
	if err := fs.Parse(args); err != nil {
		return err
	}
	c.base = strings.TrimRight(*api, "/")
	if *repo == "" {
		return c.doJSON(http.MethodGet, "/lug/v1/admin/manager/detail", nil)
	}
	return c.doJSON(http.MethodGet, "/lug/v1/admin/repos/"+url.PathEscape(*repo), nil)
}

func (c client) sync(args []string) error {
	fs := newFlagSet("sync")
	api := fs.String("api", c.base, "daemon API base URL")
	repo := fs.String("repo", "", "repo name")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *repo == "" {
		return errors.New("--repo is required")
	}
	c.base = strings.TrimRight(*api, "/")
	return c.doJSON(http.MethodPost, "/lug/v1/admin/repos/"+url.PathEscape(*repo)+"/sync", nil)
}

func (c client) simple(args []string, method, path string) error {
	fs := newFlagSet(path)
	api := fs.String("api", c.base, "daemon API base URL")
	if err := fs.Parse(args); err != nil {
		return err
	}
	c.base = strings.TrimRight(*api, "/")
	return c.doJSON(method, path, nil)
}

func newFlagSet(name string) *flag.FlagSet {
	fs := flag.NewFlagSet(name, flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	return fs
}

func (c client) doJSON(method, path string, body io.Reader) error {
	req, err := http.NewRequest(method, c.base+path, body)
	if err != nil {
		return err
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()
	payload, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	if resp.StatusCode >= 300 {
		return fmt.Errorf("%s %s: %s", method, path, strings.TrimSpace(string(payload)))
	}
	if len(bytes.TrimSpace(payload)) == 0 {
		fmt.Println(`{"status":"ok"}`)
		return nil
	}
	var pretty bytes.Buffer
	if json.Valid(payload) {
		if err := json.Indent(&pretty, payload, "", "  "); err != nil {
			return err
		}
		pretty.WriteByte('\n')
		_, err = os.Stdout.Write(pretty.Bytes())
		return err
	}
	_, err = os.Stdout.Write(payload)
	if len(payload) == 0 || payload[len(payload)-1] != '\n' {
		fmt.Println()
	}
	return err
}
