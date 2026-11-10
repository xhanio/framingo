package example

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"runtime"
	"syscall"
)

// listenSignals stops on SIGINT or SIGTERM and keeps listening while it does: the same signal
// again forces the stop, and a third exits at once.
func (m *manager) listenSignals(ctx context.Context) {
	signalCh := make(chan os.Signal, 1)
	signal.Notify(signalCh, syscall.SIGINT, syscall.SIGTERM, syscall.SIGHUP, syscall.SIGUSR1, syscall.SIGUSR2)
	defer signal.Stop(signalCh)
	done := ctx.Done()
	stopCtx, force := context.WithCancel(context.Background())
	defer force()
	var stopped chan error
	var forced bool
	for {
		select {
		case <-done:
			m.log.Info("gracefully shutdown manager")
			return
		case err := <-stopped:
			if err != nil {
				m.log.Errorf("failed to stop: %s", err)
			}
			return
		case sig := <-signalCh:
			switch sig {
			case syscall.SIGINT, syscall.SIGTERM:
				switch {
				case stopped == nil:
					m.log.Infof("received %s, shutting down (send it again to force)...", sig)
					stopped = make(chan error, 1)
					done = nil // the stop ends ctx, and stopped tells how it went
					go func() { stopped <- m.Stop(stopCtx) }()
				case !forced:
					m.log.Warnf("received %s again, forcing the shutdown (once more to exit now)...", sig)
					force()
					forced = true
				default:
					m.log.Warnf("received %s a third time, exiting now", sig)
					os.Exit(1)
				}
			case syscall.SIGHUP:
				if stopped != nil {
					m.log.Infof("received %s while shutting down, ignored", sig)
					continue
				}
				m.log.Infof("received %s, restarting all services...", sig)
				if err := m.services.Restart(ctx); err != nil {
					m.log.Errorf("failed to restart services: %s", err)
				}
			case syscall.SIGUSR1:
				m.Info(os.Stdout, true)
			case syscall.SIGUSR2:
				buf := make([]byte, 1<<20)
				n := runtime.Stack(buf, true)
				fmt.Printf("========== stack trace ==========\n\n%s\n=================================\n", buf[:n])
			}
		}
	}
}
