This repository contains a suite of benchmarks for the Go garbage
collector. These benchmarks are designed to stress specific aspects of
the garbage collector and monitor metrics of the garbage collector
itself (rather than the application).

This is a temporary home for these. They should move into the Go perf
repository once it exists.

Running the benchmarks
----------------------

The benchmarks are individual, self-contained programs in
[gcbench/progs](gcbench/progs). You can simply `go run` an individual
benchmark, or use the `buildall` script in that directory to build all
of the benchmark binaries.
