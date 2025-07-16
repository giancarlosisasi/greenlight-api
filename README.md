A project created by following the book: https://lets-go-further.alexedwards.net/

## Guides

### How to fix a broken migration with migrate

1. Fix the broken sql migration file.
2. Run the migrate command with `force {number}` to force the migration to run. It'll update the schema_migrations table but will not run the migration.
3. Go back one migration with `goto {number}`.
4. Run again the migration with `up`.