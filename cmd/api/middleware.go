package main

import (
	"fmt"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"golang.org/x/time/rate"
)

func (app *application) recoverPanic(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Create a deferred function (which will always be run in the event)
		// of a panic
		defer func() {
			// use the built-in recover() function to check if a panic occurred
			// if a panic did happen, recover() will return the panic value. If
			// a panic didn't happen, it will return nil
			pv := recover()
			if pv != nil {
				// if there was a panic, set a "Connection: close" header on the
				// response. THis acts as trigger to make Go's HTTP server
				// automatically close the current connection after the response has been
				// sent.
				w.Header().Set("Connection", "close")
				// The value returned by recover() has the type any, so we use
				// fmt.Errorf() with the %v verb to coerce it into an error and
				// call our serverErrorResponse() helper. In turn, this will log the
				// error at the ERROR level and send the client a 500 Internal
				// Server Error response
				app.serverErrorResponse(w, r, fmt.Errorf("%v", pv))
			}
		}()

		next.ServeHTTP(w, r)
	})
}

func (app *application) rateLimit(next http.Handler) http.Handler {
	if !app.config.limiter.enabled {
		return next
	}

	type client struct {
		limiter  *rate.Limiter
		lastSeen time.Time
	}

	var (
		mu      sync.Mutex
		clients = make(map[string]*client)
	)

	go func() {
		for {
			time.Sleep(time.Minute)

			mu.Lock()

			for ip, client := range clients {
				if time.Since(client.lastSeen) > 3*time.Minute {
					delete(clients, ip)
				}
			}

			mu.Lock()
		}
	}()

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ip := r.RemoteAddr

		mu.Lock()

		if _, found := clients[ip]; !found {
			clients[ip] = &client{
				limiter: rate.NewLimiter(
					rate.Limit(app.config.limiter.rps),
					app.config.limiter.burst,
				),
			}
		}

		clients[ip].lastSeen = time.Now()

		if !clients[ip].limiter.Allow() {
			mu.Unlock()
			app.rateLimitExceedResponse(w, r)
			return
		}

		// Very importantly, unlock the mutex before calling the next handler in the chain.
		// Notice that we DON'T use defer to unlock the mutex, as that would mean
		// that the mutex isn't unlocked until all the handlers downstream of this
		// middleware have also returned
		mu.Unlock()

		next.ServeHTTP(w, r)
	})
}

var trueClientIP = http.CanonicalHeaderKey("True-Client-IP")
var xForwardedFor = http.CanonicalHeaderKey("X-Forward-For")
var xRealIP = http.CanonicalHeaderKey("X-Real-IP")

func (app *application) realIP(next http.Handler) http.Handler {
	fn := func(w http.ResponseWriter, r *http.Request) {
		if rip := getRealIP(r); rip != "" {
			r.RemoteAddr = rip
		}
		next.ServeHTTP(w, r)
	}

	return http.HandlerFunc(fn)
}

func getRealIP(r *http.Request) string {
	var ip string

	if tcpi := r.Header.Get(trueClientIP); tcpi != "" {
		ip = tcpi
	} else if xrip := r.Header.Get(xRealIP); xrip != "" {
		ip = xrip
	} else if xff := r.Header.Get(xForwardedFor); xff != "" {
		ip, _, _ = strings.Cut(xff, ",")
	}

	if ip == "" || net.ParseIP(ip) == nil {
		return ""
	}

	return ip
}
