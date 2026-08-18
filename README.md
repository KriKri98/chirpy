# Chirpy

This project was build using a guide on Boot.dev.

It is a simplyfied version of X (Twitter).

You can send "Chirps" to a server, read them and create and delete users all with HTTP Requests.
The following Requests gets handled:

GET /api/healthz: Returns code 200 if server is running.
GET /admin/metrics: Returns the count of uses. Need admin credentials for this.
POST /admin/reset: Deletes all users and all chirps. Need admin credentials for this.
POST /api/chirps: Creates a Chirp and returns the Chirp.
POST /api/users: Creates a User and returns the User ID.
GET /api/chirps: Returns either all Chirps, or if author_id is present in the http query, all Chirps for a given author (User ID).
GET /api/chirps/{chirpID}: Returns the specified Chirp.
POST /api/login: Login for User. Password needs to match. Creates Refreshtoken and JWT.
POST /api/revoke: Revokes Refresh Token.
POST /api/refresh: Uses Refresh Token to create new JWT.
PUT /api/users: Updates User Email and Password.
DELETE /api/chirps/{chirpID}: Deletes specified Chirp.
POST /api/polka/webhooks: Webhook for fictional Payment processor "Polka" that upgrades a User to a paid membership.

Server is running on Localhost.

How to use it (Linux):
Install Binary via go install.
Install Postgres.
Install Goose.
Create a database for chirpy.
Create .env file in the root of your project directory.
Contents:
- DB_URL="postgres://postgres:postgres@localhost:5432/chirpy?sslmode=disable"
- PLATFORM="dev"
- JWT_SECRET="EiOIs8mlAU21z+s7HtZMq/BifunDiZP37tHj2vG8y4DWYAaBZ3HTuuGR+QYaO5gl5Qdn24HMC11RAn0SExIFbQ=="
- POLKA_KEY="f271c81ff7084ee5b99a5091b42d486e"
DB_URL will be your connection code to the database.
JWT_SECRET and POLKA_KEY are random generated keys.
Start the program in one BASH window.
Send the mentioned HTTP Requests to the server and receive data in JSON.

