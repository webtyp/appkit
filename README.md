# appkit
<img src="docs/img/badges.svg">

The application kit: one config turns WebTyp pieces (server, auth, permissions, session, menu) into a ready app

Design and decisions: [docs/ARCHITECTURE.md](docs/ARCHITECTURE.md).

- ServerConfig: Migrations []ddl.Step — «non-empty → GET /_schema (db_schema:read) serves the database diagram (webtyp.com/erd), grouped by step»
