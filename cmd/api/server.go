package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"

	// _ "net/http/pprof"
	"os"
	"os/signal"
	"syscall"
	"time"
)

func (app *application) serve() error {
	// go func() {
	// 	log.Println("pprof server starting on :6060")
	// 	log.Println(http.ListenAndServe("localhost:6060", nil))
	// }()

	srv := &http.Server{
		Addr:         fmt.Sprintf(":%d", app.config.port),
		Handler:      app.routes(),
		IdleTimeout:  time.Minute,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 10 * time.Second,
		ErrorLog:     slog.NewLogLogger(app.logger.Handler(), slog.LevelError),
	}

	// Create a shutdownError channel. We will use this to receive any errors returned
	// by the graceful Shutdown() function
	shutdownError := make(chan error)

	// start a background goroutine
	go func() {
		// create a quit channel which carries os.Signal values.
		quit := make(chan os.Signal, 1)

		// Use signal.Notify() to listen for incoming SIGINT and SIGTERM signals and
		// relay them to the quit channel. Any other signals will not be caught by
		// signal.Notify() and will retain their default behavior
		signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

		// Read the signal from the quit channel. This code will block until a signal is received
		s := <-quit

		// Log a message to say that t he signal has been caught. Notice that we also
		// call the String() method on the signal to get the signal name and include it
		// in the log entry attributes
		app.logger.Info("shutting down server", "signal", s.String())

		// Create a context with 30-second timeout
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		// Call Shutdown() on the server like before, but now we only send on the
		// Shutdown channel if it returns an error
		err := srv.Shutdown(ctx)
		if err != nil {
			shutdownError <- err
		}

		// Log a message to say that we're waiting for any background goroutines to
		// complete their tasks
		app.logger.Info("Completing background tasks", "addr", srv.Addr)

		// Call Wait() to block until our WaitGroup counter is zero ---- essentially
		// blocking until the background goroutines have finished. Then we return nil on
		// the shutdownError channel, to indicate that the shutdown completed without
		// any issues
		app.wg.Wait()
		shutdownError <- nil

		// Call Shutdown() on our server, passing in the context we just made.
		// Shutdown() will return nil if the graceful shutdown was successfully or an
		// error (which may happen because of a problem closing the listeners, or because the shutdown didn't complete
		// before the 30-second context deadline is hit). We relay this return value to the shutdownError channel
		// shutdownError <- srv.Shutdown(ctx)
	}()

	app.logger.Info("starting server", "addr", srv.Addr, "env", app.config.env)

	// Calling Shutdown() on our server will cause ListenAndServe() to immediately
	// return a http.ErrorServerClosed error. So if we see this error, it is actually a
	// god thing and an indication that the graceful shutdown has started. So we check
	// specifically for this, only returning the error if its is NOT http.ErrServerClosed.
	err := srv.ListenAndServe()
	if !errors.Is(err, http.ErrServerClosed) {
		return err
	}

	// Otherwise, we wait to receive the return value from Shutdown() on the
	// shutdownError channel. If the return value is an error, we know that there was a
	// problem with the graceful shutdown and we return the error
	err = <-shutdownError
	if err != nil {
		return err
	}

	// At this point we know that the graceful shutdown completed successfully and
	// we log a "stopped server message"
	app.logger.Info("stopped server", "addr", srv.Addr)

	return nil
}
