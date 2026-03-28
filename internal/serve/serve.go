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

	"altlinux.space/alt-atomic-lab/selenite/internal/access/keys"
)

type ServeOptions struct {
	Host        string
	Port        int
	HostKeyPath string // default: "./.selenite/keys/id_ed25519"
}

func Serve(ctx context.Context, options ServeOptions) error {
	host := options.Host
	port := options.Port

	hostKeyPath := options.HostKeyPath
	if hostKeyPath == "" {
		hostKeyPath = "./.selenite/keys/id_ed25519"
	}

	s, err := wish.NewServer(
		ssh.AllocatePty(),
		wish.WithAddress(net.JoinHostPort(host, strconv.Itoa(port))),
		wish.WithHostKeyPath(hostKeyPath),
		wish.WithPublicKeyAuth(func(ctx ssh.Context, key ssh.PublicKey) bool {
			return true
		}),
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
			func(next ssh.Handler) ssh.Handler {
				return func(sess ssh.Session) {
					keys, err := keys.NewKeysManager().List()
					if err != nil {
						_ = sess.Exit(1)
						return
					}
					if len(keys) == 0 {
						sess.Context().SetValue("is-provision-mode", true)
						next(sess)
						return
					}
					for _, key := range keys {
						if ssh.KeysEqual(sess.PublicKey(), key.Pk) {
							log.Info("new session", "name", key.Name)
							sess.Context().SetValue("is-provision-mode", false)
							sess.Context().SetValue("auth-key", key)
							next(sess)
							return
						}
					}
					wish.Println(sess, "I don't know who you are!")
					_ = sess.Exit(1)
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
