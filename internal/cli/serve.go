package cli

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/kamilandrzejrybacki-inc/vitis/internal/api"
	filestore "github.com/kamilandrzejrybacki-inc/vitis/internal/store/file"
)

// ServeCommand starts the Vitis HTTP API server.
// Flags:
//   - --port        TCP port to listen on (default 8080)
//   - --log-path    file-store root directory (default "./logs")
//   - --api-key     optional API key for Bearer auth
//   - --cors-origin optional CORS origin header value
//
// Returns 0 on clean shutdown, 1 on runtime error, 2 on config error.
func ServeCommand(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("serve", flag.ContinueOnError)
	fs.SetOutput(stderr)

	var (
		port       = fs.Int("port", 8080, "TCP port to listen on")
		logPath    = fs.String("log-path", "./logs", "file backend log root")
		apiKey     = fs.String("api-key", "", "API key for Bearer authentication (optional)")
		corsOrigin = fs.String("cors-origin", "", "CORS origin header value (optional)")
	)

	if err := fs.Parse(args); err != nil {
		return 2
	}

	if *port < 1 || *port > 65535 {
		fmt.Fprintln(stderr, "serve: --port must be in [1,65535]")
		return 2
	}

	cleanPath := filepath.Clean(*logPath)
	if cleanPath == "" || cleanPath == "." {
		fmt.Fprintln(stderr, "serve: --log-path must not be empty")
		return 2
	}

	if err := os.MkdirAll(cleanPath, 0o755); err != nil {
		fmt.Fprintf(stderr, "serve: create log directory: %v\n", err)
		return 1
	}

	store, err := filestore.New(cleanPath, false)
	if err != nil {
		fmt.Fprintf(stderr, "serve: store init: %v\n", err)
		return 1
	}
	defer store.Close()

	cfg := api.Config{
		Port:       *port,
		APIKey:     *apiKey,
		CORSOrigin: *corsOrigin,
		StoreRoot:  cleanPath,
	}

	if cfg.APIKey == "" {
		fmt.Fprintln(stderr, "WARNING: --api-key not set; API is unauthenticated")
	}

	srv, err := api.NewServer(cfg, store)
	if err != nil {
		fmt.Fprintf(stderr, "serve: server init: %v\n", err)
		return 1
	}

	fmt.Fprintf(stdout, "vitis API server listening on %s\n", srv.Addr())

	if err := srv.ListenAndServe(ctx); err != nil {
		fmt.Fprintf(stderr, "serve: %v\n", err)
		return 1
	}

	return 0
}
