// Command taskapp is the single binary: web UI, internal API and (later)
// the Jira-compatible API and MCP server.
//
// The CLI shape is part of the external contract used by the black-box API
// tests (docs/05-implementation-plan.md §3.1). Keep it stable.
package main

import (
	"bufio"
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"os/user"
	"path/filepath"
	"regexp"
	"strings"
	"syscall"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"golang.org/x/term"

	"github.com/ysksm/task-app/internal/backup"
	"github.com/ysksm/task-app/internal/domain"
	"github.com/ysksm/task-app/internal/httpapi"
	"github.com/ysksm/task-app/internal/mcpserver"
	"github.com/ysksm/task-app/internal/service"
	"github.com/ysksm/task-app/internal/store"
	"github.com/ysksm/task-app/web"
)

const usage = `taskapp - AI agent & human task manager

Usage:
  taskapp serve   [--data DIR] [--host 127.0.0.1] [--port 7700] [--require-login] [--log-level info]
                  (--host 0.0.0.0 lets other devices connect; they log in with a password)
                  [--backup-interval 24h] [--backup-keep 7] [--backup-dir DIR/backups]
  taskapp migrate [--data DIR]
  taskapp seed    [--data DIR] --preset test|demo|perf
  taskapp token create [--data DIR] --user USERNAME --name NAME
  taskapp passwd  [--data DIR] --user USERNAME [--clear]
                                              set a person's web UI password (prompted, or one line on stdin)
  taskapp mcp     [--data DIR]          MCP server over stdio; authenticates with $TASKAPP_TOKEN
  taskapp export  [--data DIR] [--out FILE]   whole database as JSON (stdout by default)
  taskapp import  [--data DIR] --in FILE      load an export into an empty data directory
  taskapp backup  [--data DIR] [--out FILE]   consistent copy of the SQLite file (safe while serving);
                                              without --out, into the backup directory like automatic ones
                                              (removes old ones only with --backup-keep N)
  taskapp restore [--data DIR] --from FILE    replace the database with a backup (server must be stopped)
  taskapp version

DIR defaults to $TASKAPP_DATA or ~/.taskapp.
`

var version = "dev"

func main() {
	if len(os.Args) < 2 {
		fmt.Fprint(os.Stderr, usage)
		os.Exit(2)
	}
	var err error
	switch os.Args[1] {
	case "serve":
		err = cmdServe(os.Args[2:])
	case "migrate":
		err = cmdMigrate(os.Args[2:])
	case "seed":
		err = cmdSeed(os.Args[2:])
	case "token":
		err = cmdToken(os.Args[2:])
	case "mcp":
		err = cmdMCP(os.Args[2:])
	case "passwd":
		err = cmdPasswd(os.Args[2:])
	case "export":
		err = cmdExport(os.Args[2:])
	case "import":
		err = cmdImport(os.Args[2:])
	case "backup":
		err = cmdBackup(os.Args[2:])
	case "restore":
		err = cmdRestore(os.Args[2:])
	case "version":
		fmt.Println(version)
	case "-h", "--help", "help":
		fmt.Print(usage)
	default:
		fmt.Fprintf(os.Stderr, "unknown command %q\n\n%s", os.Args[1], usage)
		os.Exit(2)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func defaultDataDir() string {
	if d := os.Getenv("TASKAPP_DATA"); d != "" {
		return d
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return ".taskapp"
	}
	return filepath.Join(home, ".taskapp")
}

// openService opens and migrates the database.
func openService(dataDir string) (*service.Service, func(), error) {
	db, err := store.Open(dataDir)
	if err != nil {
		return nil, nil, err
	}
	if err := store.Migrate(context.Background(), db); err != nil {
		db.Close()
		return nil, nil, fmt.Errorf("migrate: %w", err)
	}
	svc := service.New(db, nil)
	// Without the key everything still works; new tokens just cannot be shown again.
	key, err := service.LoadTokenKey(dataDir)
	if err == nil {
		err = svc.SetTokenKey(key)
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "warning: token key: %v (tokens issued now cannot be shown again later)\n", err)
	}
	return svc, func() { db.Close() }, nil
}

func cmdServe(args []string) error {
	fs := flag.NewFlagSet("serve", flag.ExitOnError)
	dataDir := fs.String("data", defaultDataDir(), "data directory")
	host := fs.String("host", "127.0.0.1", "listen host")
	port := fs.Int("port", 7700, "listen port (0 = any free port)")
	requireLogin := fs.Bool("require-login", false, "disable automatic owner login for the web UI, even on this machine")
	behindProxy := fs.Bool("behind-proxy", false, "serve through a reverse proxy on this machine (tailscale serve, Caddy): accept its Host name for MCP; implies --require-login, since every request then comes from loopback")
	publicURL := fs.String("public-url", "", "URL other devices use to reach this server (e.g. https://task.example.ts.net); shown in the settings page")
	logLevel := fs.String("log-level", "info", "debug|info|warn|error")
	logRequests := fs.Bool("log-requests", false, "log every API request (path, query, status, User-Agent); useful when connecting Jira clients")
	bk := backupFlags(fs, 7)
	backupInterval := fs.Duration("backup-interval", 24*time.Hour, "automatic backup interval (0 disables automatic backups)")
	fs.Parse(args)

	var level slog.Level
	if err := level.UnmarshalText([]byte(*logLevel)); err != nil {
		return err
	}
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: level}))

	svc, closeDB, err := openService(*dataDir)
	if err != nil {
		return err
	}
	defer closeDB()
	owner, err := svc.EnsureOwner(context.Background(), ownerName())
	if err != nil {
		return err
	}

	exe, _ := os.Executable()
	absData, _ := filepath.Abs(*dataDir)
	info := map[string]string{"version": version, "binary": exe, "data_dir": absData}
	ln, err := net.Listen("tcp", net.JoinHostPort(*host, fmt.Sprint(*port)))
	if err != nil {
		return err
	}
	tcpAddr := ln.Addr().(*net.TCPAddr)
	if !tcpAddr.IP.IsLoopback() {
		remote := remoteURL(tcpAddr)
		if remote != "" {
			info["remote_url"] = remote
		}
		logger.Warn("listening on a non-loopback address: other devices can connect. Traffic is not encrypted; use a VPN such as Tailscale, not the open internet",
			"host", *host, "url", remote)
	}
	if *publicURL != "" {
		info["remote_url"] = strings.TrimRight(*publicURL, "/")
	}
	if !tcpAddr.IP.IsLoopback() || *behindProxy {
		if n, err := svc.HumansWithPassword(context.Background()); err == nil && n == 0 {
			logger.Warn("no one has a password yet, so other devices cannot log in to the web UI; run: taskapp passwd --user " + owner.Username)
		}
	}
	backups := bk.manager(svc.DB(), *dataDir, *backupInterval, logger)
	handler := httpapi.New(svc, httpapi.Options{LocalLogin: !*requireLogin && !*behindProxy, BehindProxy: *behindProxy, Static: web.Dist(), Logger: logger, Info: info, LogRequests: *logRequests, Backups: backups, DataDir: absData})
	srv := &http.Server{Handler: handler, ReadHeaderTimeout: 10 * time.Second}

	// Contract: one JSON line on stdout when ready.
	ready, _ := json.Marshal(map[string]string{"event": "ready", "addr": ln.Addr().String()})
	fmt.Println(string(ready))
	// Let `taskapp mcp` processes on the same data directory find us.
	// Record an address they can dial even when listening on 0.0.0.0 / [::].
	local := *tcpAddr
	if local.IP.IsUnspecified() {
		local.IP = net.IPv4(127, 0, 0, 1)
	}
	serveInfo, _ := json.Marshal(map[string]string{"event": "ready", "addr": local.String()})
	serveFile := filepath.Join(*dataDir, serveInfoFile)
	if err := os.WriteFile(serveFile, serveInfo, 0o600); err == nil {
		defer os.Remove(serveFile)
	}
	logger.Info("listening", "url", "http://"+local.String(), "data", *dataDir)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	errCh := make(chan error, 1)
	go func() { errCh <- srv.Serve(ln) }()
	go backups.Run(ctx, time.Minute)
	select {
	case err := <-errCh:
		if !errors.Is(err, http.ErrServerClosed) {
			return err
		}
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		srv.Shutdown(shutdownCtx)
	}
	return nil
}

var nonUsername = regexp.MustCompile(`[^a-z0-9._-]`)

// ownerName derives the owner's username from TASKAPP_OWNER, or else from
// the OS user (a service account such as "taskapp" when run by systemd).
func ownerName() string {
	name := "owner"
	if env := os.Getenv("TASKAPP_OWNER"); env != "" {
		name = nonUsername.ReplaceAllString(strings.ToLower(env), "")
	} else if u, err := user.Current(); err == nil && u.Username != "" {
		name = nonUsername.ReplaceAllString(strings.ToLower(u.Username), "")
	}
	if name == "" {
		name = "owner"
	}
	return name
}

func cmdMigrate(args []string) error {
	fs := flag.NewFlagSet("migrate", flag.ExitOnError)
	dataDir := fs.String("data", defaultDataDir(), "data directory")
	fs.Parse(args)
	_, closeDB, err := openService(*dataDir)
	if err != nil {
		return err
	}
	closeDB()
	return nil
}

func cmdSeed(args []string) error {
	fs := flag.NewFlagSet("seed", flag.ExitOnError)
	dataDir := fs.String("data", defaultDataDir(), "data directory")
	preset := fs.String("preset", "", "test|demo")
	fs.Parse(args)
	svc, closeDB, err := openService(*dataDir)
	if err != nil {
		return err
	}
	defer closeDB()
	return svc.Seed(context.Background(), *preset)
}

func cmdToken(args []string) error {
	if len(args) == 0 || args[0] != "create" {
		return fmt.Errorf("usage: taskapp token create --user USERNAME --name NAME")
	}
	fs := flag.NewFlagSet("token create", flag.ExitOnError)
	dataDir := fs.String("data", defaultDataDir(), "data directory")
	username := fs.String("user", "", "username")
	name := fs.String("name", "", "token name")
	fs.SetOutput(io.Discard)
	if err := fs.Parse(args[1:]); err != nil {
		return err
	}
	svc, closeDB, err := openService(*dataDir)
	if err != nil {
		return err
	}
	defer closeDB()
	u, err := svc.UserByUsername(context.Background(), *username)
	if err != nil {
		return err
	}
	raw, _, err := svc.CreateToken(context.Background(), u.ID, *name, "")
	if err != nil {
		return err
	}
	// Contract: the raw token alone on one line.
	fmt.Println(raw)
	return nil
}

const serveInfoFile = "serve.json"

// cmdMCP runs an MCP server over stdio for one agent, identified by
// $TASKAPP_TOKEN. Changes are forwarded to a running `taskapp serve` (if
// any) so that the web UI updates live.
func cmdMCP(args []string) error {
	fs := flag.NewFlagSet("mcp", flag.ExitOnError)
	dataDir := fs.String("data", defaultDataDir(), "data directory")
	fs.Parse(args)
	token := os.Getenv("TASKAPP_TOKEN")
	if token == "" {
		return fmt.Errorf("set TASKAPP_TOKEN to an API token (taskapp token create --user <agent> --name mcp)")
	}
	svc, closeDB, err := openService(*dataDir)
	if err != nil {
		return err
	}
	defer closeDB()
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	u, err := svc.Authenticate(ctx, token)
	if err != nil {
		return fmt.Errorf("TASKAPP_TOKEN: %w", err)
	}
	go forwardEvents(ctx, svc, *dataDir, token)
	actor := service.Actor{User: u, Source: domain.SourceMCP}
	return mcpserver.New(svc, &actor).Run(ctx, &mcp.StdioTransport{})
}

func forwardEvents(ctx context.Context, svc *service.Service, dataDir, token string) {
	ch, unsubscribe := svc.Bus().Subscribe()
	defer unsubscribe()
	client := &http.Client{Timeout: 2 * time.Second}
	for {
		select {
		case <-ctx.Done():
			return
		case e := <-ch:
			b, err := os.ReadFile(filepath.Join(dataDir, serveInfoFile))
			if err != nil {
				continue // no server running
			}
			var info struct{ Addr string }
			if json.Unmarshal(b, &info) != nil || info.Addr == "" {
				continue
			}
			body, _ := json.Marshal(e)
			req, _ := http.NewRequestWithContext(ctx, http.MethodPost, "http://"+info.Addr+"/api/v1/internal/notify", bytes.NewReader(body))
			req.Header.Set("Authorization", "Bearer "+token)
			req.Header.Set("Content-Type", "application/json")
			if res, err := client.Do(req); err == nil {
				res.Body.Close()
			}
		}
	}
}

func cmdExport(args []string) error {
	fs := flag.NewFlagSet("export", flag.ExitOnError)
	dataDir := fs.String("data", defaultDataDir(), "data directory")
	out := fs.String("out", "", "output file (default: stdout)")
	fs.Parse(args)
	svc, closeDB, err := openService(*dataDir)
	if err != nil {
		return err
	}
	defer closeDB()
	d, err := store.Export(context.Background(), svc.DB())
	if err != nil {
		return err
	}
	if *out == "" {
		return store.WriteDump(os.Stdout, d)
	}
	f, err := os.OpenFile(*out, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	if err := store.WriteDump(f, d); err != nil {
		f.Close()
		return err
	}
	return f.Close()
}

func cmdImport(args []string) error {
	fs := flag.NewFlagSet("import", flag.ExitOnError)
	dataDir := fs.String("data", defaultDataDir(), "data directory (must be empty)")
	in := fs.String("in", "", "export file to load")
	fs.Parse(args)
	if *in == "" {
		return fmt.Errorf("--in is required")
	}
	f, err := os.Open(*in)
	if err != nil {
		return err
	}
	defer f.Close()
	d, err := store.ReadDump(f)
	if err != nil {
		return err
	}
	svc, closeDB, err := openService(*dataDir)
	if err != nil {
		return err
	}
	defer closeDB()
	counts, err := store.Import(context.Background(), svc.DB(), d)
	if err != nil {
		return err
	}
	b, _ := json.Marshal(map[string]any{"event": "imported", "rows": counts})
	fmt.Println(string(b))
	return nil
}

func cmdBackup(args []string) error {
	fs := flag.NewFlagSet("backup", flag.ExitOnError)
	dataDir := fs.String("data", defaultDataDir(), "data directory")
	out := fs.String("out", "", "backup file to create (default: a new file in the backup directory)")
	// A manual backup removes nothing unless asked to: the server may keep
	// more backups (serve --backup-keep) than this command's default.
	bk := backupFlags(fs, 0)
	fs.Parse(args)
	svc, closeDB, err := openService(*dataDir)
	if err != nil {
		return err
	}
	defer closeDB()
	if *out != "" {
		return store.Backup(context.Background(), svc.DB(), *out)
	}
	m := bk.manager(svc.DB(), *dataDir, 0, nil)
	f, err := m.Now(context.Background())
	if err != nil {
		return err
	}
	fmt.Println(filepath.Join(m.Dir(), f.Name))
	return nil
}

// backupOpts are the flags shared by serve and backup.
type backupOpts struct {
	dir  *string
	keep *int
}

func backupFlags(fs *flag.FlagSet, keep int) backupOpts {
	return backupOpts{
		dir:  fs.String("backup-dir", "", "backup directory (default: <data>/backups)"),
		keep: fs.Int("backup-keep", keep, "number of backups to keep in the backup directory (0 keeps all)"),
	}
}

func (o backupOpts) manager(db *sql.DB, dataDir string, interval time.Duration, logger *slog.Logger) *backup.Manager {
	dir := *o.dir
	if dir == "" {
		dir = filepath.Join(dataDir, "backups")
	}
	dbFile := filepath.Join(dataDir, store.DBFileName)
	return backup.New(db, backup.Config{Dir: dir, Interval: interval, Keep: *o.keep, DataFiles: []string{dbFile, dbFile + "-wal"}}, logger)
}

// cmdRestore replaces the database with a backup file. The current database
// is kept next to it as taskapp.db.before-restore-<time>.
func cmdRestore(args []string) error {
	fs := flag.NewFlagSet("restore", flag.ExitOnError)
	dataDir := fs.String("data", defaultDataDir(), "data directory")
	from := fs.String("from", "", "backup file (.db) to restore")
	fs.Parse(args)
	if *from == "" {
		return fmt.Errorf("--from is required")
	}
	if addr := runningServer(*dataDir); addr != "" {
		return fmt.Errorf("taskapp serve is running on %s with this data directory; stop it first", addr)
	}
	aside, err := store.Restore(context.Background(), *dataDir, *from)
	if aside != "" {
		fmt.Fprintln(os.Stderr, "previous database saved as", aside)
	}
	if err != nil {
		return err
	}
	fmt.Fprintln(os.Stderr, "restored", *from)
	return nil
}

// runningServer returns the address of a `taskapp serve` using dataDir, if
// one answers.
func runningServer(dataDir string) string {
	b, err := os.ReadFile(filepath.Join(dataDir, serveInfoFile))
	if err != nil {
		return ""
	}
	var info struct{ Addr string }
	if json.Unmarshal(b, &info) != nil || info.Addr == "" {
		return ""
	}
	c, err := net.DialTimeout("tcp", info.Addr, time.Second)
	if err != nil {
		return "" // left behind by a server that did not shut down cleanly
	}
	c.Close()
	return info.Addr
}

// remoteURL is the URL other devices use: the listen address itself, or for
// 0.0.0.0 / [::] the first private IPv4 address of this machine.
func remoteURL(addr *net.TCPAddr) string {
	if !addr.IP.IsUnspecified() {
		return "http://" + addr.String()
	}
	ifaces, err := net.InterfaceAddrs()
	if err != nil {
		return ""
	}
	var fallback string
	for _, a := range ifaces {
		n, ok := a.(*net.IPNet)
		if !ok || n.IP.To4() == nil || n.IP.IsLoopback() || n.IP.IsLinkLocalUnicast() {
			continue
		}
		u := fmt.Sprintf("http://%s", net.JoinHostPort(n.IP.String(), fmt.Sprint(addr.Port)))
		if n.IP.IsPrivate() {
			return u
		}
		if fallback == "" {
			fallback = u
		}
	}
	return fallback
}

func cmdPasswd(args []string) error {
	fs := flag.NewFlagSet("passwd", flag.ExitOnError)
	dataDir := fs.String("data", defaultDataDir(), "data directory")
	username := fs.String("user", "", "username of a person (agents use API tokens)")
	clear := fs.Bool("clear", false, "remove the password (the user can then only log in on this machine, if owner)")
	fs.Parse(args)
	if *username == "" {
		return fmt.Errorf("--user is required")
	}
	svc, closeDB, err := openService(*dataDir)
	if err != nil {
		return err
	}
	defer closeDB()
	password := ""
	if !*clear {
		if password, err = readPassword(); err != nil {
			return err
		}
		if password == "" {
			return fmt.Errorf("empty password (use --clear to remove it)")
		}
	}
	if _, err := svc.SetPasswordDirect(context.Background(), *username, password); err != nil {
		return err
	}
	if *clear {
		fmt.Fprintln(os.Stderr, "password removed for", *username)
	} else {
		fmt.Fprintln(os.Stderr, "password set for", *username, "(takes effect immediately, also on a running server)")
	}
	return nil
}

// readPassword prompts twice on a terminal, or reads one line from stdin.
func readPassword() (string, error) {
	fd := int(os.Stdin.Fd())
	if !term.IsTerminal(fd) {
		line, err := bufio.NewReader(os.Stdin).ReadString('\n')
		if err != nil && line == "" {
			return "", fmt.Errorf("read password from stdin: %w", err)
		}
		return strings.TrimRight(line, "\r\n"), nil
	}
	fmt.Fprint(os.Stderr, "New password: ")
	a, err := term.ReadPassword(fd)
	fmt.Fprintln(os.Stderr)
	if err != nil {
		return "", err
	}
	fmt.Fprint(os.Stderr, "Again: ")
	b, err := term.ReadPassword(fd)
	fmt.Fprintln(os.Stderr)
	if err != nil {
		return "", err
	}
	if string(a) != string(b) {
		return "", fmt.Errorf("the passwords do not match")
	}
	return string(a), nil
}
