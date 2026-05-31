// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.

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
	gossh "golang.org/x/crypto/ssh"

	"altlinux.space/alt-atomic-lab/selenite/internal/access"
	v1 "altlinux.space/alt-atomic-lab/selenite/internal/machine/config/v1"
)

type ServeOptions struct {
	Host        string
	Port        int
	HostKeyPath string // default: "./.selenite/keys/id_ed25519"
}

// publicKeyAuth returns true if key matches any configured user, or if no users
// are configured (provision mode).
func publicKeyAuth(_ ssh.Context, key ssh.PublicKey) bool {
	cfg, err := v1.ConfigStore{}.Load()
	if err != nil || len(cfg.Access.Users) == 0 {
		return true // provision mode: accept any key
	}
	for _, user := range cfg.Access.Users {
		for _, keyStr := range user.Keys {
			pk, _, _, _, err := gossh.ParseAuthorizedKey([]byte(keyStr))
			if err == nil && ssh.KeysEqual(key, pk) {
				return true
			}
		}
	}
	return false
}

// findUserByKey returns the first user whose authorized keys include key, or nil.
func findUserByKey(users []v1.User, key gossh.PublicKey) *v1.User {
	for i, user := range users {
		for _, keyStr := range user.Keys {
			pk, _, _, _, err := gossh.ParseAuthorizedKey([]byte(keyStr))
			if err == nil && ssh.KeysEqual(key, pk) {
				return &users[i]
			}
		}
	}
	return nil
}

// cmdMiddleware runs the cli app for each SSH session.
func cmdMiddleware(next ssh.Handler) ssh.Handler {
	return func(sess ssh.Session) {
		ctx, cancel := context.WithCancel(sess.Context())

		app := AppCmd(sess)
		if err := app.Run(ctx, append([]string{""}, sess.Command()...)); err != nil {
			_, _ = fmt.Fprintf(app.ErrWriter, "Error: %s\n", err)
			cancel()
			_ = sess.Exit(1)
			return
		}

		cancel()
		next(sess)
	}
}

// authMiddleware resolves which user owns the session key and injects RBAC
// context values, or sets provision-mode if no users are configured yet.
func authMiddleware(next ssh.Handler) ssh.Handler {
	return func(sess ssh.Session) {
		cfg, err := v1.ConfigStore{}.Load()
		if err != nil {
			_ = sess.Exit(1)
			return
		}

		if len(cfg.Access.Users) == 0 {
			sess.Context().SetValue(ctxIsProvisionMode, true)
			next(sess)
			return
		}

		matchedUser := findUserByKey(cfg.Access.Users, sess.PublicKey())
		if matchedUser == nil {
			wish.Println(sess, "I don't know who you are!")
			_ = sess.Exit(1)
			return
		}

		log.Info("new session", "name", matchedUser.Name)

		enforcer, err := access.NewEnforcer(cfg.Access)
		if err != nil {
			_ = sess.Exit(1)
			return
		}

		sess.Context().SetValue(ctxIsProvisionMode, false)
		sess.Context().SetValue(ctxRBACUsername, matchedUser.Name)
		sess.Context().SetValue(ctxRBACEnforcer, enforcer)

		next(sess)
	}
}

func Serve(ctx context.Context, options ServeOptions) error {
	hostKeyPath := options.HostKeyPath
	if hostKeyPath == "" {
		hostKeyPath = "./.selenite/keys/id_ed25519"
	}

	s, err := wish.NewServer(
		ssh.AllocatePty(),
		wish.WithAddress(net.JoinHostPort(options.Host, strconv.Itoa(options.Port))),
		wish.WithHostKeyPath(hostKeyPath),
		wish.WithPublicKeyAuth(publicKeyAuth),
		wish.WithMiddleware(
			cmdMiddleware,
			authMiddleware,
			logging.Middleware(),
		),
	)
	if err != nil {
		return fmt.Errorf("failed to create server: %w", err)
	}

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	log.Info("Starting SSH server", "host", options.Host, "port", options.Port)
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
