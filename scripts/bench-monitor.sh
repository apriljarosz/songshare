#!/bin/bash

# Benchmark monitoring script - tracks performance over time

BENCH_DIR="benchmarks"
HISTORY_FILE="$BENCH_DIR/history.json"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Create benchmarks directory if it doesn't exist
mkdir -p "$BENCH_DIR"

# Function to run benchmarks and save results
run_benchmark() {
    local timestamp=$(date +%Y%m%d_%H%M%S)
    local output_file="$BENCH_DIR/bench_$timestamp.txt"
    
    echo -e "${GREEN}Running benchmarks...${NC}"
    go test -bench=. -benchmem -run=^$ ./... > "$output_file"
    
    echo -e "${GREEN}Benchmark results saved to: $output_file${NC}"
    return 0
}

# Function to compare with previous benchmark
compare_with_previous() {
    local latest=$(ls -t "$BENCH_DIR"/bench_*.txt 2>/dev/null | head -1)
    local previous=$(ls -t "$BENCH_DIR"/bench_*.txt 2>/dev/null | head -2 | tail -1)
    
    if [ -z "$previous" ]; then
        echo -e "${YELLOW}No previous benchmark found for comparison${NC}"
        return 1
    fi
    
    echo -e "${GREEN}Comparing with previous benchmark...${NC}"
    echo "Previous: $previous"
    echo "Current: $latest"
    
    if command -v benchstat >/dev/null 2>&1; then
        benchstat "$previous" "$latest"
    else
        echo -e "${YELLOW}benchstat not installed. Install with: go install golang.org/x/perf/cmd/benchstat@latest${NC}"
        echo "Showing simple diff:"
        diff -u "$previous" "$latest" | grep -E "^[+-]Benchmark" || true
    fi
}

# Function to show benchmark trends
show_trends() {
    echo -e "${GREEN}Benchmark History (last 5 runs):${NC}"
    ls -t "$BENCH_DIR"/bench_*.txt 2>/dev/null | head -5 | while read file; do
        echo "  - $(basename $file)"
        # Extract and show summary stats
        grep -E "^Benchmark.*ns/op" "$file" | head -3 | sed 's/^/    /'
    done
}

# Function to run specific benchmark
run_specific() {
    local pattern=$1
    local timestamp=$(date +%Y%m%d_%H%M%S)
    local output_file="$BENCH_DIR/bench_${pattern}_$timestamp.txt"
    
    echo -e "${GREEN}Running benchmark: $pattern${NC}"
    go test -bench="$pattern" -benchmem -run=^$ ./... > "$output_file"
    
    echo -e "${GREEN}Results saved to: $output_file${NC}"
    tail -10 "$output_file"
}

# Function to generate performance report
generate_report() {
    local report_file="$BENCH_DIR/performance_report_$(date +%Y%m%d).md"
    
    echo -e "${GREEN}Generating performance report...${NC}"
    
    cat > "$report_file" << EOF
# Performance Report - $(date +"%Y-%m-%d")

## Summary
Generated on: $(date)

## Current Benchmark Results

\`\`\`
$(go test -bench=. -benchmem -run=^$ -benchtime=10s ./internal/services/... 2>/dev/null | grep Benchmark)
\`\`\`

## Repository & Cache Performance

\`\`\`
$(go test -bench=. -benchmem -run=^$ -benchtime=10s ./internal/repositories/... ./internal/cache/... 2>/dev/null | grep Benchmark)
\`\`\`

## Handler Performance

\`\`\`
$(go test -bench=. -benchmem -run=^$ -benchtime=10s ./internal/handlers/... 2>/dev/null | grep Benchmark)
\`\`\`

## Performance Trends

### Last 5 Runs
$(ls -t "$BENCH_DIR"/bench_*.txt 2>/dev/null | head -5 | while read file; do
    echo "- $(basename $file): $(grep -c "^Benchmark" $file) benchmarks"
done)

## Recommendations

Based on the benchmark results:
1. Monitor memory allocations in hot paths
2. Consider caching strategies for frequently accessed data
3. Review concurrent operations for potential optimizations

EOF
    
    echo -e "${GREEN}Report saved to: $report_file${NC}"
}

# Main menu
show_menu() {
    echo -e "\n${GREEN}Benchmark Monitor${NC}"
    echo "1. Run all benchmarks"
    echo "2. Compare with previous run"
    echo "3. Show benchmark trends"
    echo "4. Run specific benchmark"
    echo "5. Generate performance report"
    echo "6. Clean old benchmarks"
    echo "7. Exit"
    echo -n "Choose an option: "
}

# Clean old benchmarks (keep last 10)
clean_old() {
    echo -e "${YELLOW}Keeping last 10 benchmark files...${NC}"
    ls -t "$BENCH_DIR"/bench_*.txt 2>/dev/null | tail -n +11 | xargs -r rm
    echo -e "${GREEN}Cleanup complete${NC}"
}

# Process command line arguments
if [ $# -gt 0 ]; then
    case "$1" in
        run)
            run_benchmark
            ;;
        compare)
            run_benchmark
            compare_with_previous
            ;;
        trends)
            show_trends
            ;;
        report)
            generate_report
            ;;
        specific)
            if [ -z "$2" ]; then
                echo "Usage: $0 specific <pattern>"
                exit 1
            fi
            run_specific "$2"
            ;;
        *)
            echo "Usage: $0 [run|compare|trends|report|specific <pattern>]"
            exit 1
            ;;
    esac
    exit 0
fi

# Interactive mode
while true; do
    show_menu
    read choice
    
    case $choice in
        1)
            run_benchmark
            ;;
        2)
            compare_with_previous
            ;;
        3)
            show_trends
            ;;
        4)
            echo -n "Enter benchmark pattern (e.g., BenchmarkSongResolution): "
            read pattern
            run_specific "$pattern"
            ;;
        5)
            generate_report
            ;;
        6)
            clean_old
            ;;
        7)
            echo "Exiting..."
            exit 0
            ;;
        *)
            echo -e "${RED}Invalid option${NC}"
            ;;
    esac
done