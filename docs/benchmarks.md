# Glamour rendering benchmarks

These benchmarks are performance-regression checks for terminal Markdown
rendering. They are intentionally not strict pass/fail tests: compare results
with `benchstat` or medians from repeated runs.

Recommended local commands:

```bash
go test ./... -run '^$' -bench . -benchmem
scripts/sciagent-markdown-bench.sh
```

The SciAgent script runs the integration benchmarks that use this checkout via
SciAgent's local `replace ../glamour` module setting and can optionally collect
CPU and allocation profiles.
