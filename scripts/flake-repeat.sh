#!/usr/bin/env bash
# Run one Go test many times under CPU contention, to reproduce an
# intermittent failure. The test, everything it starts, and the load are all
# confined to a few CPUs with taskset, at nice 19, so a shared machine's other
# users keep the rest of it. See development/2026-11-12-test-flakiness/plan.md.
#
#   scripts/flake-repeat.sh [options] PACKAGE TEST_REGEX
#
#   --runs N       test processes to run (default 10)
#   --count N      -test.count in each process (default 5)
#   --parallel N   test processes at a time (default 2)
#   --cpus LIST    taskset CPU list (default 0-3)
#   --load N       busy loops per CPU in LIST (default 2; 0 for none)
#   --race         build the test binary with -race
#   --out DIR      keep output in DIR (default: a new directory under $TMPDIR)
#
# Each process's output is kept as run-K.log; the logs of processes that
# passed are removed unless --keep-passing is given. The summary counts the
# top-level "--- PASS" and "--- FAIL" lines of the tests TEST_REGEX matches.
set -euo pipefail

runs=10
count=5
parallel=2
cpus=0-3
load=2
race=false
out=""
keep_passing=false

usage() {
    sed -n '/^#   scripts/,/^# top-level/p' "$0" | sed 's/^# \{0,1\}//' >&2
}

while (($# > 0)); do
    case "$1" in
    --runs) runs=$2 && shift 2 ;;
    --count) count=$2 && shift 2 ;;
    --parallel) parallel=$2 && shift 2 ;;
    --cpus) cpus=$2 && shift 2 ;;
    --load) load=$2 && shift 2 ;;
    --race) race=true && shift ;;
    --out) out=$2 && shift 2 ;;
    --keep-passing) keep_passing=true && shift ;;
    -h | --help)
        usage
        exit 0
        ;;
    -*)
        usage
        exit 2
        ;;
    *) break ;;
    esac
done
if (($# != 2)); then
    usage
    exit 2
fi
package=$1
pattern=$2

command -v taskset >/dev/null || {
    echo "taskset is required" >&2
    exit 1
}

root=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
package_dir=$root/${package#./}
[[ -d "$package_dir" ]] || {
    echo "no package directory $package_dir" >&2
    exit 1
}
[[ -n "$out" ]] || out=$(mktemp -d "${TMPDIR:-/tmp}/flake-repeat-XXXXXX")
mkdir -p "$out"

# The number of CPUs in LIST, by expanding it the way taskset does.
cpu_count=$(taskset -c "$cpus" nproc)

build_flags=()
[[ "$race" == true ]] && build_flags+=(-race)
binary=$out/test.bin
(cd "$root" && go test -c "${build_flags[@]}" -o "$binary" "./${package#./}")

loaders=()
stop_load() {
    ((${#loaders[@]} == 0)) || kill "${loaders[@]}" 2>/dev/null || true
}
trap stop_load EXIT
for ((i = 0; i < load * cpu_count; i++)); do
    taskset -c "$cpus" nice -n 19 sh -c 'while :; do :; done' &
    loaders+=($!)
done

{
    echo "commit:   $(cd "$root" && jj log -r @- --no-graph -T 'commit_id.short() ++ " " ++ description.first_line()' 2>/dev/null || echo unknown)"
    echo "package:  $package"
    echo "test:     $pattern"
    echo "runs:     $runs x count $count, $parallel at a time"
    echo "cpus:     $cpus ($cpu_count), busy loops $((load * cpu_count)), race $race"
    echo "started:  $(date -u +%FT%TZ)"
} | tee "$out/summary.txt"

run_one() {
    local index=$1 log=$out/run-$1.log tmp=$out/tmp-$1
    mkdir -p "$tmp"
    local code=0
    (cd "$package_dir" && TMPDIR=$tmp taskset -c "$cpus" nice -n 19 \
        "$binary" -test.run "$pattern" -test.count "$count" -test.v -test.timeout 30m) >"$log" 2>&1 || code=$?
    echo "exit $code" >>"$log"
    rm -rf "$tmp"
    if [[ "$code" == 0 && "$keep_passing" != true ]]; then
        # Keep the counts of a passing log, not its output.
        grep -E '^--- (PASS|FAIL)' "$log" >"$log.counts" || true
        rm -f "$log"
    fi
}

# Wait only for test processes: the busy loops never exit.
workers=()
for ((index = 1; index <= runs; index++)); do
    run_one "$index" &
    workers+=($!)
    while (($(jobs -pr | grep -cvxFf <(printf '%s\n' "${loaders[@]:-none}")) >= parallel)); do
        sleep 0.2
    done
done
for worker in "${workers[@]}"; do
    wait "$worker" || true
done
stop_load

pass=$(cat "$out"/run-*.log* 2>/dev/null | grep -c '^--- PASS' || true)
fail=$(cat "$out"/run-*.log* 2>/dev/null | grep -c '^--- FAIL' || true)
failed_logs=$(grep -l '^exit [1-9]' "$out"/run-*.log 2>/dev/null || true)
{
    echo "finished: $(date -u +%FT%TZ)"
    echo "result:   $pass passed, $fail failed"
    if [[ -n "$failed_logs" ]]; then
        echo "failing processes:"
        for log in $failed_logs; do
            echo "  $log"
            grep -E '^\s*--- FAIL' "$log" | sort | uniq -c | sed 's/^/    /'
        done
    fi
    echo "output:   $out"
} | tee -a "$out/summary.txt"
[[ -z "$failed_logs" ]]
