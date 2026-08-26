// migrate applies the SQL schema to the database.
//
// It uses the golang-migrate library instead of the CLI binary, so nobody on
// the team needs to install anything extra. `make migrate-up` just works.
//
//	go run ./cmd/migrate --config ./config/development.env up
//	go run ./cmd/migrate --config ./config/development.env down 1
//	go run ./cmd/migrate create add_start_command
package main

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/golang-migrate/migrate/v4"
	migratepg "github.com/golang-migrate/migrate/v4/database/postgres"
	"github.com/golang-migrate/migrate/v4/source/iofs"
	"github.com/mistryviral05/dockyard/internal/config"
	postgres "github.com/mistryviral05/dockyard/internal/http/database"
)

const defaultMigrationsDir = "db/migrations/postgres"

const (
	colorReset = "\033[0m"
	colorDim   = "\033[90m"
	colorGreen = "\033[32m"
	colorRed   = "\033[31m"
	colorCyan  = "\033[36m"
)

func main() {
	// Note: config.MustLoad registers its own `config` flag. We register it
	// here first and hand the value over through CONFIG_PATH, otherwise the
	// second registration panics with "flag redefined".
	configPath := flag.String("config", "", "path to the config file (.yaml or .env)")
	dir := flag.String("path", defaultMigrationsDir, "migrations folder")
	flag.Usage = usage
	flag.Parse()

	args := flag.Args()
	if len(args) == 0 {
		usage()
		os.Exit(2)
	}
	command, rest := args[0], args[1:]

	// create only writes files, so it never touches the database.
	if command == "create" {
		if len(rest) != 1 {
			fail("create needs a name, for example: create add_start_command")
		}
		if err := createMigration(*dir, rest[0]); err != nil {
			fail(err.Error())
		}
		return
	}

	m, closeFn := open(*configPath, *dir)
	defer closeFn()

	switch command {
	case "up":
		run(m, m.Up, "every pending migration was applied")

	case "down":
		if len(rest) > 0 && rest[0] == "all" {
			run(m, m.Down, "every migration was rolled back")
			break
		}
		steps := 1
		if len(rest) > 0 {
			steps = mustInt(rest[0])
		}
		run(m, func() error { return m.Steps(-steps) },
			fmt.Sprintf("rolled back %d migration(s)", steps))

	case "goto":
		if len(rest) != 1 {
			fail("goto needs a version number, for example: goto 5")
		}
		target := mustInt(rest[0])
		run(m, func() error { return m.Migrate(uint(target)) },
			fmt.Sprintf("now at version %d", target))

	case "version":
		printVersion(m)

	case "force":
		if len(rest) != 1 {
			fail("force needs a version number, for example: force 8")
		}
		target := mustInt(rest[0])
		if err := m.Force(target); err != nil {
			fail(err.Error())
		}
		okf("version %d is marked clean again", target)
		printVersion(m)

	default:
		fmt.Fprintf(os.Stderr, "unknown command: %s\n\n", command)
		usage()
		os.Exit(2)
	}
}

// open reads the config, connects to the database and builds the migrator.
func open(configPath, dir string) (*migrate.Migrate, func()) {
	if configPath != "" {
		os.Setenv("CONFIG_PATH", configPath)
	}
	// MustLoad calls flag.Parse() internally and would choke on our
	// subcommand arguments, so hand it a clean command line.
	flag.CommandLine = flag.NewFlagSet(os.Args[0], flag.ExitOnError)
	os.Args = os.Args[:1]

	cfg := config.MustLoad()

	if _, err := os.Stat(dir); err != nil {
		fail(fmt.Sprintf("migrations folder not found: %s", dir))
	}

	src, err := iofs.New(os.DirFS(dir), ".")
	if err != nil {
		fail(fmt.Sprintf("could not read the migration files: %s", err))
	}

	db, err := postgres.New(cfg.Database)
	if err != nil {
		fail(fmt.Sprintf("could not connect to the database: %s", err))
	}

	driver, err := migratepg.WithInstance(db, &migratepg.Config{})
	if err != nil {
		db.Close()
		fail(fmt.Sprintf("could not build the migration driver: %s", err))
	}

	m, err := migrate.NewWithInstance("iofs", src, "postgres", driver)
	if err != nil {
		db.Close()
		fail(fmt.Sprintf("could not build the migrator: %s", err))
	}
	m.Log = logger{}

	fmt.Printf("%s> %s@%s:%d/%s  (%s)%s\n",
		colorDim, cfg.Database.User, cfg.Database.Host, cfg.Database.Port, cfg.Database.Name, dir, colorReset)

	return m, func() { db.Close() }
}

func run(m *migrate.Migrate, fn func() error, done string) {
	err := fn()
	switch {
	case errors.Is(err, migrate.ErrNoChange):
		okf("nothing changed, the database is already up to date")
	case errors.Is(err, os.ErrNotExist):
		okf("there is no migration left in that direction")
	case err != nil:
		fail(err.Error())
	default:
		okf("%s", done)
	}
	printVersion(m)
}

func printVersion(m *migrate.Migrate) {
	version, dirty, err := m.Version()
	if errors.Is(err, migrate.ErrNilVersion) {
		fmt.Printf("%scurrent version: none, no migration has been applied yet%s\n", colorDim, colorReset)
		return
	}
	if err != nil {
		fail(err.Error())
	}
	state := "clean"
	color := colorCyan
	if dirty {
		state = "DIRTY, recover with the force command"
		color = colorRed
	}
	fmt.Printf("%scurrent version: %d (%s)%s\n", color, version, state, colorReset)
}

// createMigration picks the next sequence number and writes an empty up and
// down pair.
func createMigration(dir, name string) error {
	slug := strings.ToLower(strings.NewReplacer(" ", "_").Replace(strings.TrimSpace(name)))
	if slug == "" {
		return errors.New("a migration name cannot be empty")
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		return fmt.Errorf("could not read the migrations folder: %w", err)
	}

	next := 1
	for _, e := range entries {
		base := e.Name()
		idx := strings.IndexByte(base, '_')
		if idx <= 0 {
			continue
		}
		if n, err := strconv.Atoi(base[:idx]); err == nil && n >= next {
			next = n + 1
		}
	}

	for _, direction := range []string{"up", "down"} {
		path := filepath.Join(dir, fmt.Sprintf("%06d_%s.%s.sql", next, slug, direction))
		body := fmt.Sprintf("-- %s: %s\n", direction, slug)
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			return fmt.Errorf("could not write the file: %w", err)
		}
		okf("created %s", path)
	}
	return nil
}

func mustInt(s string) int {
	n, err := strconv.Atoi(s)
	if err != nil || n < 0 {
		fail(fmt.Sprintf("%q should be a number", s))
	}
	return n
}

func okf(format string, a ...any) {
	fmt.Printf("%s[ok] %s%s\n", colorGreen, fmt.Sprintf(format, a...), colorReset)
}

func fail(msg string) {
	fmt.Fprintf(os.Stderr, "%s[fail] %s%s\n", colorRed, msg, colorReset)
	os.Exit(1)
}

// logger dims the messages that golang-migrate prints while it works.
type logger struct{}

func (logger) Printf(format string, v ...any) {
	fmt.Printf("%s  %s%s", colorDim, fmt.Sprintf(format, v...), colorReset)
}

func (logger) Verbose() bool { return true }

func usage() {
	fmt.Fprintf(os.Stderr, `dockyard migrate

Usage:
  go run ./cmd/migrate [flags] <command> [args]

Commands:
  up                apply every pending migration
  down [N]          roll back the last N migrations (default 1)
  down all          roll back everything
  goto <version>    move straight to that version
  version           show the version the database is on
  force <version>   clear a dirty state
  create <name>     write a new empty up and down pair

Flags:
  --config <path>   config file (.yaml or .env). Without it the config is read
                    from environment variables.
  --path <dir>      migrations folder (default %s)
`, defaultMigrationsDir)
}
