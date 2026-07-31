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
	"time"

	"charm.land/log/v2"
	"charm.land/ssh"
	"charm.land/wish/v2"
	"charm.land/wish/v2/logging"
	gossh "golang.org/x/crypto/ssh"

	"altlinux.space/alt-atomic/selenite/internal/access"
	v1 "altlinux.space/alt-atomic/selenite/internal/machine/config/v1"
	"altlinux.space/alt-atomic/selenite/internal/serve/authguard"
)

type ServeOptions struct {
	Host        string
	Port        int
	HostKeyPath string // default: "./.selenite/keys/id_ed25519"
}

func extractIP(addr net.Addr) string {
	host, _, err := net.SplitHostPort(addr.String())
	if err != nil {
		return addr.String()
	}
	return host
}

// publicKeyAuthInner returns true if key matches any configured user, or if no users
// are configured (provision mode).
func publicKeyAuthInner(_ ssh.Context, key ssh.PublicKey) bool {
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

// publicKeyAuth returns a handler that checks authguard before delegating to
// publicKeyAuthInner. Failed attempts are recorded and may lead to a ban.
func publicKeyAuth(guard *authguard.Guard) ssh.PublicKeyHandler {
	return publicKeyAuthWithInner(guard, publicKeyAuthInner)
}

func publicKeyAuthWithInner(guard *authguard.Guard, inner ssh.PublicKeyHandler) ssh.PublicKeyHandler {
	return func(ctx ssh.Context, key ssh.PublicKey) bool {
		ip := extractIP(ctx.RemoteAddr())
		if !guard.Allow(ip) {
			return false
		}
		ok := inner(ctx, key)
		if !ok {
			if guard.RecordFailure(ip) {
				log.Info("authguard banned IP after repeated failures", "ip", ip)
			}
		}
		return ok
	}
}

// banConnCallback rejects TCP connections from banned IPs before the SSH handshake.
func banConnCallback(guard *authguard.Guard) ssh.ConnCallback {
	return func(_ ssh.Context, conn net.Conn) net.Conn {
		ip := extractIP(conn.RemoteAddr())
		if !guard.Allow(ip) {
			log.Info("authguard rejecting banned IP", "ip", ip)
			_ = conn.Close()
			return nil
		}
		return conn
	}
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
			log.Warn("failed to load config", "err", err)
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
			log.Warn("failed to create new enforces", "err", err)
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

	cfg, err := v1.ConfigStore{}.Load()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	guard, err := authguard.NewGuard(cfg.Access.AuthGuard)
	if err != nil {
		return fmt.Errorf("failed to init authguard: %w", err)
	}

	s, err := wish.NewServer(
		ssh.AllocatePty(),
		wish.WithAddress(net.JoinHostPort(options.Host, strconv.Itoa(options.Port))),
		wish.WithHostKeyPath(hostKeyPath),
		wish.WithPublicKeyAuth(publicKeyAuth(guard)),
		wish.WithMiddleware(
			cmdMiddleware,
			authMiddleware,
			logging.Middleware(),
		),
		func(srv *ssh.Server) error {
			srv.ConnCallback = banConnCallback(guard)
			return nil
		},
	)
	if err != nil {
		return fmt.Errorf("failed to create server: %w", err)
	}

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	go guard.CleanupLoop(ctx, 5*time.Minute)

	log.Info("Starting SSH server", "host", options.Host, "port", options.Port)
	go func() {
		if err = s.ListenAndServe(); err != nil && !errors.Is(err, ssh.ErrServerClosed) {
			log.Error("Could not start server", "error", err)
			cancel()
		}
	}()

	<-ctx.Done()
	log.Info("Stopping SSH server")

	if err := guard.Save(); err != nil {
		log.Warn("failed to save authguard state", "err", err)
	}
	return nil
}
