# Command Entry Points

This directory contains the main entry points for the different binaries of the system.

- `benchmark/`: Tools to measure the throughput and latency of the logging pipeline.
- `orchestrator/`: The main control plane that manages log flow and coordinates workers.
- `worker/`: Processing units that handle log ingestion and forward them to the storage engine.
- `ioctl/`: System-level control tools for managing the engine state.


## Verify the wiring works

After you’ve edited the two files, run:

```bash
# rebuilds agent & server binaries
make clean && make build    

# Then start a quick test:
# start rust engine (already built)
./rust_engine/target/x86_64-unknown-linux-musl/release/rust_engine &

# start server with custom values
./bin/release/grpc-server -batch-size=40000 -flush-interval=100ms &

# start a couple of agents with different sleeps
./bin/release/grpc-agent -sleep=500us &
./bin/release/grpc-agent -sleep=200us &
```

Watch the orchestrator’s stdout – you’ll see lines like:


📈 Requests/sec: 3421.12 (total in last second: 3421)


If you ever see ❌ CRITICAL: Pipeline buffer full, dropping metric! you know the offered load exceeds the current batchSize / flushInterval capacity; simply increase -batch-size or decrease -flush-interval (or run more server instances) and rebuild/restart.



TL;DR – What you get

| Component    | Configurable knob                                  | Default | How to set                                                 |
|--------------|----------------------------------------------------|---------|------------------------------------------------------------|
| Agent        | push interval (-sleep or AGENT_SLEEP)              | 1ms     | -sleep=200us or AGENT_SLEEP=50ms ./grpc-agent              |
| Orchestrator | batch size (-batch-size or BATCH_SIZE)             | 20 000  | -batch-size=40000 or BATCH_SIZE=80000 ./grpc-server        |
|              | flush interval (-flush-interval or FLUSH_INTERVAL) | 500ms   | -flush-interval=100ms or FLUSH_INTERVAL=20ms ./grpc-server |