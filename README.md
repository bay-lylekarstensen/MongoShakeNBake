# Mongo-Shake-N-Bake

## Fork Disclaimer
Mongo-Shake-N-Bake is an independent fork of Alibaba's Mongo-Shake.

This fork was created specifically to fix and harden mongodb+srv connection support for cloud hosting providers that rely on SRV connection strings for replica set discovery. It also adds a safe config test mode so operators can validate settings before running replication.

This software is provided AS-IS, without warranty of any kind.

Upstream project: https://github.com/alibaba/MongoShake

## Why This Fork Exists
The primary focus of Mongo-Shake-N-Bake is reliable SRV URI support across the collector configuration paths used in real-world managed MongoDB environments, especially cloud providers where `mongodb+srv://` is the standard way to reach replica sets.

In addition, this fork improves operator safety with a preflight config test mode.

## What Is Different From Upstream
1. Primary enhancement: robust `mongodb+srv://` handling for:
   - source URLs (`mongo_urls`)
   - direct tunnel target URL (`tunnel.address` when `tunnel=direct`)
   - checkpoint storage URL (`checkpoint.storage.url`)
2. Critical bugfix: URL parsing logic was fixed to prevent mangling of query parameters in MongoDB connection strings.
3. Operational enhancement: `-check-config` mode to validate config and connectivity without starting replication.
4. Fork-owned issue flow: bugs and feature requests should be opened in this fork's issue tracker, not upstream.

## Quick Start
Build and run:

```bash
git clone https://github.com/bay-lylekarstensen/MongoShakeNBake.git
cd MongoShakeNBake
make
./bin/collector -conf=conf/collector.conf
```

Run directly with Go (no `make` required):

```bash
git clone https://github.com/bay-lylekarstensen/MongoShakeNBake.git
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

## Reporting Issues
Please report issues for this fork here:

https://github.com/bay-lylekarstensen/MongoShakeNBake/issues

Do not open fork-specific issues in the upstream Alibaba repository.

## Contributing
Contributions are welcome in this fork. Open a pull request against this repository.

## Credits
Mongo-Shake-N-Bake is built on top of Mongo-Shake, originally developed and maintained by the Alibaba Cloud NoSQL team and community contributors.

Thank you to the original maintainers and contributors for creating and open-sourcing Mongo-Shake.

## License
This fork continues to use the original MIT license.

See LICENSE for the full license text. Do not remove or replace upstream copyright/license notices.
