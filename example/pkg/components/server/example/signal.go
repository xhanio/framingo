package example

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"runtime"
	"syscall"
)

func (m *manager) listenSignals(ctx context.Context) {
	signalCh := make(chan os.Signal, 1)
	signal.Notify(signalCh, syscall.SIGINT, syscall.SIGTERM, syscall.SIGHUP, syscall.SIGUSR1, syscall.SIGUSR2)
	defer signal.Stop(signalCh)
	for {
		select {
		case <-ctx.Done():
			m.log.Info("gracefully shutdown manager")
			return
		case sig := <-signalCh:
			switch sig {
			case syscall.SIGINT, syscall.SIGTERM:
				m.log.Infof("received %s, shutting down...", sig)
				if err := m.Stop(true); err != nil {
					m.log.Errorf("failed to stop: %s", err)
				}
				return
			case syscall.SIGHUP:
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
