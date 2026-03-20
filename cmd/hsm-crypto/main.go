package main

import (
	"os"
	"os/signal"
	"syscall"

	"github.com/PayWithSpireInc/transaction-processing/internal/appbase"
)

func main() {
	app := appbase.New(
		appbase.Init("hsm-crypto"),
		appbase.WithDependencyInjector(),
	)
	defer app.Shutdown()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	<-stop
}
