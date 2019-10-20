# BlogEngine

This is the server written to serve my blog https://satran.in. It is custom built to run my blog completely from memory.

If you plan to use it there are a few environment variables you need to provide:

- `HOSTNAME`: to ensure the right hostname is set for serving RSS feeds
- `CERT` & `KEY`: if you intend on using TLS, the files for the certificate
- `SITE`: the directory from which to serve the site from
- `METRICS_TOKEN`: a random string to be used as token if you want to serve it under `/metrics` for prometheus


The site directory needs to follow the structure below:

- `blog/`: all your articles written as markdown. The first line is the Title and the second line is the date with a format `dd/mm/yyyy`. Blog URLS are prefixed with `/b` rather than `/blog`.
- `static/`: directory serving static files. Static URLs are prefixed with `/s` rather than `/static`
- `alias.json`: a JSON key value object with the key being the alias of the value. I created this as I wanted to migrate the URLs of my old site.
