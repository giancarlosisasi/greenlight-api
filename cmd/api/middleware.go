package main

import (
	"errors"
	"fmt"
	"net"
	"net/http"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/giancarlosisasi/greenlight-api/internal/data"
	"github.com/giancarlosisasi/greenlight-api/internal/validator"
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

			mu.Unlock()
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

func (app *application) authenticate(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Add the Vary: Authorization header to the response.
		// This indicates to any caches that the response may vary based on the value of
		// the Authorization header in the request
		w.Header().Add("Vary", "Authorization")

		// Retrieve the value of the Authorization header from the request.
		// This will return the empty string "" if there is not such header found.
		authorizationHeader := r.Header.Get("Authorization")

		if authorizationHeader == "" {
			r = app.contextSetUser(r, data.AnonymousUser)
			next.ServeHTTP(w, r)
			return
		}

		// Otherwise, we expect the value of the Authorization header to be in the format
		// "Bearer <token>". We try to split this into its constituent parts, and if the
		// header isn't in the expected format we return 401 Unauthorized response
		// using the invalidAuthenticationTokenResponse() helper
		headerParts := strings.Split(authorizationHeader, " ")
		if len(headerParts) != 2 || headerParts[0] != "Bearer" {
			app.invalidCredentialsResponse(w, r)
			return
		}

		token := headerParts[1]

		v := validator.New()

		if data.ValidateTokenPlainText(v, token); !v.Valid() {
			app.invalidAuthenticationTokenResponse(w, r)
			return
		}

		user, err := app.models.Users.GetForToken(data.ScopeAuthentication, token)
		if err != nil {
			switch {
			case errors.Is(err, data.ErrRecordNotFound):
				app.invalidAuthenticationTokenResponse(w, r)
			default:
				app.serverErrorResponse(w, r, err)
			}
			return
		}

		r = app.contextSetUser(r, user)

		next.ServeHTTP(w, r)
	})
}

func (app *application) requireAuthenticatedUser(next http.HandlerFunc) http.HandlerFunc {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user := app.contextGetUser(r)

		if user.IsAnonymous() {
			app.authenticationRequiredResponse(w, r)
			return
		}

		next.ServeHTTP(w, r)
	})
}

func (app *application) requireActivatedUser(next http.HandlerFunc) http.HandlerFunc {
	fn := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user := app.contextGetUser(r)

		if !user.Activated {
			app.inactiveAccountResponse(w, r)
			return
		}

		next.ServeHTTP(w, r)
	})

	return app.requireAuthenticatedUser(fn)
}

func (app *application) requirePermissions(code string, next http.HandlerFunc) http.HandlerFunc {
	fn := func(w http.ResponseWriter, r *http.Request) {
		user := app.contextGetUser(r)

		permissions, err := app.models.Permissions.GetAllForUser(user.ID)
		if err != nil {
			app.serverErrorResponse(w, r, err)
			return
		}

		if !permissions.Include(code) {
			app.notPermittedResponse(w, r)
			return
		}

		next.ServeHTTP(w, r)
	}

	return app.requireActivatedUser(fn)
}

func (app *application) enableCORS(next http.Handler) http.Handler {
	trustedOrigins := []string{
		"http://localhost:9000",
		"http://localhost:9002",
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Add("Vary", "Origin")
		w.Header().Add("Vary", "Access-Control-Request-Method")

		origin := r.Header.Get("Origin")

		if origin == "" {
			next.ServeHTTP(w, r)
			return
		}

		if slices.Contains(trustedOrigins, origin) {
			w.Header().Set("Access-Control-Allow-Origin", origin)

			// CHeck if the request hast the HTTP method OPTIONS and contains the
			// "Access-Control-Request-Method" header. If it does, then we treat
			// it as a preflight request.
			if r.Method == http.MethodOptions && r.Header.Get("Access-Control-Request-Method") != "" {
				// Set the necessary preflight response headers
				w.Header().Set("Access-Control-Allow-Methods", "OPTIONS, PUT, PATCH, DELETE")
				w.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type")

				// write the headers along with a 200 ok status and return from
				// the middleware with no further action
				// set 200 ok and not 204 because some browsers doesn't support 204 code
				w.WriteHeader(http.StatusOK)
			}
		}

		next.ServeHTTP(w, r)

	})
}
