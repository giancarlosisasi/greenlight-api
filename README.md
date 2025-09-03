A project created by following the book: https://lets-go-further.alexedwards.net/

## Guides

### How to fix a broken migration with migrate

1. Fix the broken sql migration file.
2. Run the migrate command with `force {number}` to force the migration to run. It'll update the schema_migrations table but will not run the migration.
3. Go back one migration with `goto {number}`.
4. Run again the migration with `up`.

### How to debug deadlocks and more:

1. in the `server.go` file, import `_ "net/http/pprof"`
2. Add this to the `serve` function:

```go
go func() {
	log.Println("pprof server starting on :6060")
	log.Println(http.ListenAndServe("localhost:6060", nil))
}()
```

continue: file:///D:/Books/golang/Lets%20go%20further/lets-go-further.html/lets-go-further.html/16.01-requiring-user-activation.html # Splitting up the middleware