# Glyphtones
### A simple audio-sharing platform for Nothing Phone users

![Screenshots](https://s3-nothing-prod.s3.eu-central-1.amazonaws.com/2025-01-04/1735987786-859251-render.png)

### Tech stack
The app uses [Go](https://go.dev/) + [echo](https://echo.labstack.com/) + [templ](https://github.com/a-h/templ) to render HTML pages for the client (and a little bit of [htmx](https://htmx.org/)). This approach is called "server-side rendering". Data is stored in a [PostgreSQL](https://www.postgresql.org/) database. 

### Production
The website is running in Germany, Falkenstein on [Hetzner](https://www.hetzner.com/cloud/) VPS.

I didn't want to pay for a domain so I used this sweet [is-a.dev](https://is-a.dev/) project.

### How to run (for developers)
1. Install Go compiler and PostgreSQL server
2. Create a new database in psql
3. Run the _init.sql_ file to setup the database
4. Clone this repository
5. Rename _.env.example_ to _.env_ and configure your enviroment variables
6. Run `go run .` in the terminal
7. The site should be running on http://localhost:1323