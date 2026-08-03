1. Install postgres
2. Set Password (Linux): sudo passwd postgres
3. Start Server sudo: service postgresql start
4. Connect to Server: sudo -u postgres psql
5. Create Database and connect to it \c
6. ALTER USER postgres WITH PASSWORD 'postgres';
7. Install goose
8. Create files in sql/schema
9. Write Migrations in sql/schema, e.g. 001_users.sql (Comments -- +goose Up/Down)
10. Get connection string: postgres://postgres:postgres@localhost:5432/DATABASE
11. Test connection string: psql "postgres://wagslane:@localhost:5432/DATABASE"
12. Migrate: goose postgres "postgres://wagslane:@localhost:5432/DATABASE" up
13. Install SQLC
14. Create sqlc.yaml in the root of the project:
version: "2"
sql:
  - schema: "sql/schema"
    queries: "sql/queries"
    engine: "postgresql"
    gen:
      go:
        out: "internal/database"
15. Write Queries to sql/queries e.g. users.sql
16. Let SQLC create go code for the queries: sqlc generate
