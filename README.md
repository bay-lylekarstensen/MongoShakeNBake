# Mongo-Shake-N-Bake

## Fork Disclaimer
Mongo-Shake-N-Bake is an independent fork of Alibaba's Mongo-Shake.

This fork was created to make Mongo-Shake safer and more practical in managed/cloud environments. The main goals are robust `mongodb+srv://` support, safer preflight validation, and environment-driven configuration for secret-friendly deployments.

This software is provided AS-IS, without warranty of any kind.

Upstream project: https://github.com/alibaba/MongoShake

## Why This Fork Exists
The primary focus of Mongo-Shake-N-Bake is reliable SRV URI support across the collector configuration paths used in real-world managed MongoDB environments, especially cloud providers where `mongodb+srv://` is the standard way to reach replica sets.

In addition, this fork improves operator safety with preflight validation and operational flexibility with environment variable expansion and `.env` autoload.

This fork also improves full-sync operational safety by supporting replace-mode duplicate handling and background reconciliation, reducing or eliminating the need to drop and recreate destination data to achieve parity.

## What Is Different From Upstream
1. Primary enhancement: robust `mongodb+srv://` handling for:
   - source URLs (`mongo_urls`)
   - direct tunnel target URL (`tunnel.address` when `tunnel=direct`)
   - checkpoint storage URL (`checkpoint.storage.url`)
2. Critical bugfix: URL parsing logic was fixed to prevent mangling of query parameters in MongoDB connection strings.
3. Operational enhancement: `-check-config` mode validates config and connectivity without starting replication.
4. Deployment enhancement: config values support environment variable expansion (`$VAR` / `${VAR}`), including source, destination, and checkpoint URLs.
5. Deployment enhancement: if a `.env` file exists in project root, it is auto-loaded before config parsing.
6. Data parity enhancement: full-sync duplicate handling supports `update` and `replace` modes, and an optional background reconcile worker can remove target-only documents over time, so operators do not need to rely on drop-and-recreate workflows.
7. Fork-owned issue flow: bugs and feature requests should be opened in this fork's issue tracker, not upstream.

## Quick Start
Build and run:

```bash
git clone https://github.com/back-at-you-inc/MongoShakeNBake.git
cd MongoShakeNBake
make
./bin/collector -conf=conf/collector.conf
```

Run directly with Go (no `make` required):

```bash
git clone https://github.com/back-at-you-inc/MongoShakeNBake.git
cd MongoShakeNBake
go run ./cmd/collector -conf=conf/collector.conf
```

## Test Your Config Before Running
Validate configuration and connectivity without starting sync:

```bash
./bin/collector -conf=conf/collector.conf -check-config
```

If running directly with Go:

```bash
go run ./cmd/collector -conf=conf/collector.conf -check-config
```

This preflight mode verifies core connectivity settings and exits.

## Configuration Notes
Mongo-Shake-N-Bake supports both `mongodb://` and `mongodb+srv://` connection strings.

For direct writes, ensure `tunnel=direct` and `tunnel.address` are set correctly.

Config values now support environment variable expansion before parsing.
Supported forms: `$VAR` and `${VAR}`.
If a `.env` file exists in the process working directory (project root), it is auto-loaded first.
This behavior is applied by the shared config loader used by both `collector` and `receiver`.

Example:

```ini
mongo_urls = ${MSB_SOURCE_URL}
tunnel.address = ${MSB_DEST_ADDRESS}
checkpoint.storage.url = ${MSB_CHECKPOINT_URL}
```

Run with env vars:

```bash
export MSB_SOURCE_URL='mongodb+srv://user:pass@source.example.net/?tls=true&authSource=admin'
export MSB_DEST_ADDRESS='mongodb+srv://user:pass@dest.example.net/?tls=true&authSource=admin'
export MSB_CHECKPOINT_URL="$MSB_SOURCE_URL"
./bin/collector -conf=conf/collector.conf -check-config
```

Note: unset variables are expanded to an empty string.

Duplicate-key handling modes for insert collisions:

- `full_sync.executor.insert_on_dup_update = true` enables full-sync duplicate handling.
- `full_sync.executor.insert_on_dup_update_mode = update|replace` controls behavior (default: `update`).
- `incr_sync.executor.insert_on_dup_update = true` enables incremental duplicate handling.
- `incr_sync.executor.insert_on_dup_update_mode = update|replace` controls behavior (default: `update`).

Mode behavior:

- `update`: applies `$set` with source fields, preserving extra fields already present on target documents.
- `replace`: replaces the full target document with the source document (using `_id`/document key filter), which is better when you need strict document-shape parity.

Background reconcile worker (eventual delete parity without collection drop):

- `full_sync.reconcile.enable = true` enables a secondary reconcile process.
- The worker scans source `_id` keys, records seen state in reconcile state collections on target, and prunes target-only docs in batches.
- Deletions are guarded by `full_sync.reconcile.grace_runs`, so docs must be missing for multiple runs before removal.

Key settings:

- `full_sync.reconcile.interval` (seconds)
- `full_sync.reconcile.delete_batch_size`
- `full_sync.reconcile.grace_runs`
- `full_sync.reconcile.db`
- `full_sync.reconcile.collection`

Storage layout:

- State is written into `full_sync.reconcile.db`.
- Collection names are generated per namespace using `full_sync.reconcile.collection` as a prefix.

Compatibility note:

- `full_sync.reconcile.shadow_db` and `full_sync.reconcile.shadow_collection` are still accepted as legacy aliases.

Notes:

- In `sync_mode=all`, reconcile runs as a background worker after full sync completes.
- In `sync_mode=full`, reconcile runs once before process exit when enabled.

## Docker Usage
The repository Docker image uses a multi-stage build:

- Builder stage: Go toolchain compiles binaries with `make linux`.
- Runtime stage: lightweight Alpine image runs `collector` by default.

Container entrypoint behavior:

- Entrypoint script: `docker-entrypoint.sh`.
- Default command: runs `collector`.
- Default config path: `/app/conf/collector.conf`.
- Runtime config override env: `MSB_CONF`.

`MSB_CONF` resolution rules:

- If `MSB_CONF` contains `/`, it is treated as a path and used as-is.
- If `MSB_CONF` has no `/`, it is treated as a filename under `/app/conf`.

Examples:

```bash
# uses /app/conf/collector.conf
docker run --rm <image>

# uses /app/conf/collector-atlas.conf (filename mode)
docker run --rm -e MSB_CONF=collector-atlas.conf <image>

# uses exact path (path mode)
docker run --rm -e MSB_CONF=/custom/collector.conf <image>

# pass collector flags while keeping MSB_CONF behavior
docker run --rm -e MSB_CONF=collector-atlas.conf <image> -check-config
```

Duplicate-handling log output (full sync):

- Collision fallback warning (expected path when duplicate keys exist):

```text
Full sync insert encountered duplicate keys and will resolve via mode[replace]. ns[{db.collection}] attempted[N] inserted[M] duplicates[D]
```

- Completion info (explicit replacement result):

```text
Full sync duplicate resolution mode[replace] ns[{db.collection}] attempted[N] inserted[M] replaced[R] done
```

## Reporting Issues
Please report issues for this fork here:

https://github.com/back-at-you-inc/MongoShakeNBake/issues

Do not open fork-specific issues in the upstream Alibaba repository.

## Contributing
Contributions are welcome in this fork. Open a pull request against this repository.

## Credits
Mongo-Shake-N-Bake is built on top of Mongo-Shake, originally developed and maintained by the Alibaba Cloud NoSQL team and community contributors.

Thank you to the original maintainers and contributors for creating and open-sourcing Mongo-Shake.

## License
This fork continues to use the original MIT license.

See LICENSE for the full license text. Do not remove or replace upstream copyright/license notices.
