# Frontend domain types

`generated.ts` is **generated from the Go domain models** (`server/internal/models`)
with [tygo](https://github.com/gzuidhof/tygo) — it is the single source of truth for
API payload/response shapes and must not be edited by hand.

## Regenerate

```bash
npm run gen:types          # from frontend/
# or:  cd server && go run github.com/gzuidhof/tygo@v0.2.21 generate
```

Config lives in [`server/tygo.yaml`](../../../server/tygo.yaml) (`time.Time → string`).
Commit the regenerated `generated.ts` alongside any model change.

## Layout

- **`generated.ts`** — auto-generated model interfaces (DO NOT EDIT).
- **Domain files** (`host.ts`, `proxmox.ts`, `docker.ts`, `tracker.ts`, `task.ts`, …)
  re-export the generated model types and add what generation can't express:
  - refinements (e.g. `HostStatus` narrows `Host.status` from `string` to a union),
  - response **envelopes** that aren't Go models (e.g. `DockerContainersPage`),
  - flattened shapes for embedded structs tygo nests instead of inlining
    (e.g. `ScheduledTaskWithHost = ScheduledTask & { host_name }`).

## Caveats (tygo conventions)

- Every Go pointer field becomes optional `?: T` (undefined) — including
  non-`omitempty` pointers that serialise as `null`. Treat such fields as
  possibly absent.
- Anonymous embedded structs are emitted as a **nested** property, not inlined —
  redefine those as an intersection in the domain file.
- Request vs response shapes differ for some domains (e.g. `ProxmoxConnection`
  has no `token_secret`; use `ProxmoxConnectionRequest` for create/update bodies).
