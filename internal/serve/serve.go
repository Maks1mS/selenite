package serve

import (
	"context"
	"errors"
	"fmt"
	"net"
	"strconv"

	"charm.land/log/v2"
	"charm.land/wish/v2"
	"charm.land/wish/v2/logging"
	"github.com/charmbracelet/ssh"
)

type ServeOptions struct {
	Host string
	Port int
}

func Serve(ctx context.Context, options ServeOptions) error {
	host := options.Host
	port := options.Port

	s, err := wish.NewServer(
		ssh.AllocatePty(),
		wish.WithAddress(net.JoinHostPort(host, strconv.Itoa(port))),
		wish.WithHostKeyPath("./.selenite/keys/id_ed25519"),
		wish.WithMiddleware(
			func(next ssh.Handler) ssh.Handler {
				return func(sess ssh.Session) {
					app := appCmd(sess)
					if err := app.Run(sess.Context(), append([]string{""}, sess.Command()...)); err != nil {
						_, _ = fmt.Fprintf(app.ErrWriter, "Error: %s\n", err)
						_ = sess.Exit(1)
						return
					}
					next(sess)
				}
			},
			logging.Middleware(),
		),
	)
	if err != nil {
		return fmt.Errorf("failed to create server: %w", err)
	}

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	log.Info("Starting SSH server", "host", host, "port", port)
	go func() {
		if err = s.ListenAndServe(); err != nil && !errors.Is(err, ssh.ErrServerClosed) {
			log.Error("Could not start server", "error", err)
			cancel()
		}
	}()

	<-ctx.Done()
	log.Info("Stopping SSH server")

	return nil
}
