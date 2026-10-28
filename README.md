# appkit
<img src="docs/img/badges.svg">

The application kit: one config turns WebTyp pieces (server, auth, permissions, session, menu) into a ready app

Design and decisions: [docs/ARCHITECTURE.md](docs/ARCHITECTURE.md).

| `ServerConfig` field | What |
|---|---|
| `Migrations []ddl.Step` | non-empty → `GET /_schema` (`db_schema:read`, never public) serves the app's database diagram ([`webtyp.com/erd`](https://github.com/webtyp/erd)), one band per migration step |
