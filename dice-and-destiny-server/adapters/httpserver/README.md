# HTTP Server Adapter

Reserved for a future server-authoritative version of the battle authority.

This adapter would expose the same portable Go authority over HTTP or WebSocket-style server transport.

It should call the same `internal/battle` packages used by the local GDExtension adapter.

Do not implement this until there is a concrete story requiring server-authoritative transport.
