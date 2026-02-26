package app

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"runtime"
	"syscall"
)

func defaultSignals(m *manager) map[os.Signal]func() {
	return map[os.Signal]func(){
		syscall.SIGINT: func() {
			m.log.Info("received SIGINT, shutting down...")
			if err := m.Stop(true); err != nil {
				m.log.Errorf("failed to stop: %s", err)
			}
		},
		syscall.SIGTERM: func() {
			m.log.Info("received SIGTERM, shutting down...")
			if err := m.Stop(true); err != nil {
				m.log.Errorf("failed to stop: %s", err)
			}
		},
		syscall.SIGUSR1: func() {
			m.Info(os.Stdout, true)
		},
		syscall.SIGUSR2: func() {
			buf := make([]byte, 1<<20)
			n := runtime.Stack(buf, true)
			fmt.Printf("========== stack trace ==========\n\n%s\n=================================\n", buf[:n])
		},
	}
}

func (m *manager) listenSignals(ctx context.Context) {
	signalCh := make(chan os.Signal, 1)
	sigs := make([]os.Signal, 0, len(m.signals))
	for sig := range m.signals {
		sigs = append(sigs, sig)
	}
	signal.Notify(signalCh, sigs...)
	go func() {
		defer signal.Stop(signalCh)
		for {
			select {
			case <-ctx.Done():
				return
			case sig := <-signalCh:
				if handler, ok := m.signals[sig]; ok {
					handler()
				} else {
					m.log.Warnf("unhandled signal: %s", sig)
				}
			}
		}
	}()
}
