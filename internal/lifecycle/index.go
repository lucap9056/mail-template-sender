package lifecycle

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"
)

type Shutdown struct {
	ctx    context.Context
	cancel context.CancelFunc
}

func New() *Shutdown {
	ctx, cancel := context.WithCancel(context.Background())

	s := &Shutdown{
		ctx:    ctx,
		cancel: cancel,
	}

	go s.handleSignals()

	return s
}

func (s *Shutdown) handleSignals() {
	signalChan := make(chan os.Signal, 1)
	signal.Notify(signalChan, syscall.SIGINT, syscall.SIGTERM)

	<-signalChan
	s.cancel()
}

func (s *Shutdown) Wait() {
	<-s.ctx.Done()
}

func (s *Shutdown) Shutdown(format string, v ...any) {
	if format != "" {
		log.Println(format, v)
	}

	s.cancel()
}
