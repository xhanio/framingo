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
	signal.Notify(signalCh, syscall.SIGINT, syscall.SIGTERM, syscall.SIGUSR1, syscall.SIGUSR2)
	go func() {
		defer signal.Stop(signalCh)
		for {
			select {
			case <-ctx.Done():
				return
			case sig := <-signalCh:
				switch sig {
				case syscall.SIGINT, syscall.SIGTERM:
					m.log.Infof("received %s, shutting down...", sig)
					if err := m.services.Stop(true); err != nil {
						m.log.Errorf("failed to stop: %s", err)
					}
					m.cancel()
				case syscall.SIGUSR1:
					m.Info(os.Stdout, true)
				case syscall.SIGUSR2:
					buf := make([]byte, 1<<20)
					n := runtime.Stack(buf, true)
					fmt.Printf("========== stack trace ==========\n\n%s\n=================================\n", buf[:n])
				}
			}
		}
	}()
}
