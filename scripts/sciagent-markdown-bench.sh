#!/usr/bin/env bash
set -euo pipefail

script_dir="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
glamour_dir="$(cd -- "$script_dir/.." && pwd)"
sciagent_dir="${SCIAGENT_DIR:-$(cd -- "$glamour_dir/../sciagent-llm-unify" && pwd)}"
logs_dir="${MARKDOWN_BENCH_LOGS:-$sciagent_dir/logs/markdown-bench}"

mkdir -p "$logs_dir"

(
  cd "$sciagent_dir"
  go test ./tui/render \
    -run '^$' \
    -bench 'Benchmark(MarkdownRendering|RenderAssistantMessageLiveLongTail)$' \
    -benchmem \
    -count="${COUNT:-5}" \
    -cpuprofile "$logs_dir/cpu-after.out" \
    -memprofile "$logs_dir/mem-after.out" \
    | tee "$logs_dir/bench-after.txt"

  go tool pprof -top "$logs_dir/cpu-after.out" > "$logs_dir/cpu-after-top.txt"
  go tool pprof -top -alloc_space "$logs_dir/mem-after.out" > "$logs_dir/mem-after-alloc-space-top.txt"
  go tool pprof -top -alloc_objects "$logs_dir/mem-after.out" > "$logs_dir/mem-after-alloc-objects-top.txt"
)
