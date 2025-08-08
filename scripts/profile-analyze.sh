#!/bin/bash

# Performance profiling and analysis script

PROFILE_DIR="benchmarks/profiles"
FLAMEGRAPH_DIR="$PROFILE_DIR/flamegraphs"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Create directories
mkdir -p "$PROFILE_DIR" "$FLAMEGRAPH_DIR"

# Function to run CPU profiling
profile_cpu() {
    echo -e "${GREEN}Running CPU profiling...${NC}"
    
    local timestamp=$(date +%Y%m%d_%H%M%S)
    local profile_file="$PROFILE_DIR/cpu_$timestamp.prof"
    
    go test -bench=. -benchmem -cpuprofile="$profile_file" -run=^$ ./...
    
    echo -e "${GREEN}CPU profile saved to: $profile_file${NC}"
    echo -e "${BLUE}View with: go tool pprof $profile_file${NC}"
    
    # Generate text report
    echo -e "${YELLOW}Generating CPU profile report...${NC}"
    go tool pprof -text "$profile_file" > "$PROFILE_DIR/cpu_report_$timestamp.txt"
    echo -e "${GREEN}CPU report saved to: $PROFILE_DIR/cpu_report_$timestamp.txt${NC}"
}

# Function to run memory profiling
profile_memory() {
    echo -e "${GREEN}Running memory profiling...${NC}"
    
    local timestamp=$(date +%Y%m%d_%H%M%S)
    local profile_file="$PROFILE_DIR/mem_$timestamp.prof"
    
    go test -bench=. -benchmem -memprofile="$profile_file" -run=^$ ./...
    
    echo -e "${GREEN}Memory profile saved to: $profile_file${NC}"
    echo -e "${BLUE}View with: go tool pprof $profile_file${NC}"
    
    # Generate text report
    echo -e "${YELLOW}Generating memory profile report...${NC}"
    go tool pprof -text "$profile_file" > "$PROFILE_DIR/mem_report_$timestamp.txt"
    echo -e "${GREEN}Memory report saved to: $PROFILE_DIR/mem_report_$timestamp.txt${NC}"
}

# Function to profile a specific package
profile_package() {
    local package=$1
    local timestamp=$(date +%Y%m%d_%H%M%S)
    
    if [ -z "$package" ]; then
        echo -e "${RED}Package not specified${NC}"
        return 1
    fi
    
    echo -e "${GREEN}Profiling package: $package${NC}"
    
    local cpu_profile="$PROFILE_DIR/cpu_${package}_$timestamp.prof"
    local mem_profile="$PROFILE_DIR/mem_${package}_$timestamp.prof"
    
    # CPU profiling
    echo -e "${YELLOW}Running CPU profiling for $package...${NC}"
    go test -bench=. -benchmem -cpuprofile="$cpu_profile" -run=^$ ./$package/...
    
    # Memory profiling
    echo -e "${YELLOW}Running memory profiling for $package...${NC}"
    go test -bench=. -benchmem -memprofile="$mem_profile" -run=^$ ./$package/...
    
    echo -e "${GREEN}Profiles saved:${NC}"
    echo "  CPU: $cpu_profile"
    echo "  Memory: $mem_profile"
}

# Function to analyze existing profiles
analyze_profile() {
    local profile_file=$1
    
    if [ ! -f "$profile_file" ]; then
        echo -e "${RED}Profile file not found: $profile_file${NC}"
        return 1
    fi
    
    echo -e "${GREEN}Analyzing profile: $profile_file${NC}"
    
    # Generate various reports
    local base_name=$(basename "$profile_file" .prof)
    local report_dir="$PROFILE_DIR/analysis_$(date +%Y%m%d_%H%M%S)"
    mkdir -p "$report_dir"
    
    echo -e "${YELLOW}Generating analysis reports...${NC}"
    
    # Top 20 functions
    go tool pprof -text -nodecount=20 "$profile_file" > "$report_dir/${base_name}_top20.txt"
    
    # Call graph (top 10)
    go tool pprof -dot -nodecount=10 "$profile_file" > "$report_dir/${base_name}_graph.dot"
    
    # List functions
    go tool pprof -list=".*" "$profile_file" > "$report_dir/${base_name}_list.txt"
    
    echo -e "${GREEN}Analysis complete. Reports saved in: $report_dir${NC}"
    
    # Show top 10 functions
    echo -e "${BLUE}Top 10 functions:${NC}"
    head -15 "$report_dir/${base_name}_top20.txt"
}

# Function to generate flame graphs (if available)
generate_flamegraph() {
    local profile_file=$1
    
    if [ ! -f "$profile_file" ]; then
        echo -e "${RED}Profile file not found: $profile_file${NC}"
        return 1
    fi
    
    # Check if go-torch is available
    if ! command -v go-torch >/dev/null 2>&1; then
        echo -e "${YELLOW}go-torch not found. Install with: go install github.com/uber/go-torch@latest${NC}"
        return 1
    fi
    
    local base_name=$(basename "$profile_file" .prof)
    local flamegraph_file="$FLAMEGRAPH_DIR/${base_name}_$(date +%Y%m%d_%H%M%S).svg"
    
    echo -e "${GREEN}Generating flame graph...${NC}"
    go-torch -b "$profile_file" -f "$flamegraph_file"
    
    echo -e "${GREEN}Flame graph saved to: $flamegraph_file${NC}"
    echo -e "${BLUE}Open with: open $flamegraph_file${NC}"
}

# Function to run comprehensive profiling
comprehensive_profile() {
    echo -e "${GREEN}Running comprehensive profiling...${NC}"
    
    local timestamp=$(date +%Y%m%d_%H%M%S)
    local report_dir="$PROFILE_DIR/comprehensive_$timestamp"
    mkdir -p "$report_dir"
    
    # Profile each major component
    local packages=("internal/services" "internal/repositories" "internal/handlers" "internal/cache")
    
    for package in "${packages[@]}"; do
        echo -e "${YELLOW}Profiling $package...${NC}"
        
        # CPU profile
        go test -bench=. -benchmem -cpuprofile="$report_dir/cpu_$(basename $package).prof" -run=^$ ./$package/... 2>/dev/null
        
        # Memory profile  
        go test -bench=. -benchmem -memprofile="$report_dir/mem_$(basename $package).prof" -run=^$ ./$package/... 2>/dev/null
        
        echo -e "${GREEN}$package profiling complete${NC}"
    done
    
    # Generate summary report
    cat > "$report_dir/README.md" << EOF
# Comprehensive Profiling Report
Generated: $(date)

## Profile Files
$(ls -1 $report_dir/*.prof | while read file; do echo "- $(basename $file)"; done)

## Usage
\`\`\`bash
# View CPU profile
go tool pprof $report_dir/cpu_services.prof

# View memory profile  
go tool pprof $report_dir/mem_repositories.prof

# Generate flame graph
go-torch -b $report_dir/cpu_services.prof
\`\`\`

## Quick Analysis
$(for prof in $report_dir/cpu_*.prof; do
    echo "### $(basename $prof)"
    echo "\`\`\`"
    go tool pprof -text -nodecount=5 "$prof" 2>/dev/null | head -10
    echo "\`\`\`"
    echo ""
done)
EOF
    
    echo -e "${GREEN}Comprehensive profiling complete. Report: $report_dir/README.md${NC}"
}

# Function to clean old profiles
clean_profiles() {
    echo -e "${YELLOW}Cleaning old profile files...${NC}"
    
    # Keep last 5 of each type
    ls -t "$PROFILE_DIR"/cpu_*.prof 2>/dev/null | tail -n +6 | xargs -r rm
    ls -t "$PROFILE_DIR"/mem_*.prof 2>/dev/null | tail -n +6 | xargs -r rm
    ls -t "$PROFILE_DIR"/cpu_report_*.txt 2>/dev/null | tail -n +6 | xargs -r rm
    ls -t "$PROFILE_DIR"/mem_report_*.txt 2>/dev/null | tail -n +6 | xargs -r rm
    
    echo -e "${GREEN}Cleanup complete${NC}"
}

# Main menu
show_menu() {
    echo -e "\n${GREEN}Performance Profiler${NC}"
    echo "1. CPU profiling (all tests)"
    echo "2. Memory profiling (all tests)"
    echo "3. Profile specific package"
    echo "4. Analyze existing profile"
    echo "5. Generate flame graph"
    echo "6. Comprehensive profiling"
    echo "7. Clean old profiles"
    echo "8. List available profiles"
    echo "9. Exit"
    echo -n "Choose an option: "
}

# List available profiles
list_profiles() {
    echo -e "${GREEN}Available profiles:${NC}"
    
    echo -e "${BLUE}CPU Profiles:${NC}"
    ls -t "$PROFILE_DIR"/cpu_*.prof 2>/dev/null | head -5 | while read file; do
        echo "  - $(basename $file) ($(date -r $file))"
    done
    
    echo -e "${BLUE}Memory Profiles:${NC}"
    ls -t "$PROFILE_DIR"/mem_*.prof 2>/dev/null | head -5 | while read file; do
        echo "  - $(basename $file) ($(date -r $file))"
    done
}

# Command line interface
if [ $# -gt 0 ]; then
    case "$1" in
        cpu)
            profile_cpu
            ;;
        memory|mem)
            profile_memory
            ;;
        package)
            if [ -z "$2" ]; then
                echo "Usage: $0 package <package-path>"
                exit 1
            fi
            profile_package "$2"
            ;;
        analyze)
            if [ -z "$2" ]; then
                echo "Usage: $0 analyze <profile-file>"
                exit 1
            fi
            analyze_profile "$2"
            ;;
        flamegraph)
            if [ -z "$2" ]; then
                echo "Usage: $0 flamegraph <profile-file>"
                exit 1
            fi
            generate_flamegraph "$2"
            ;;
        comprehensive)
            comprehensive_profile
            ;;
        clean)
            clean_profiles
            ;;
        list)
            list_profiles
            ;;
        *)
            echo "Usage: $0 [cpu|memory|package|analyze|flamegraph|comprehensive|clean|list]"
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
            profile_cpu
            ;;
        2)
            profile_memory
            ;;
        3)
            echo -n "Enter package path (e.g., internal/services): "
            read package
            profile_package "$package"
            ;;
        4)
            echo "Available profiles:"
            ls "$PROFILE_DIR"/*.prof 2>/dev/null | head -5
            echo -n "Enter profile file path: "
            read profile_file
            analyze_profile "$profile_file"
            ;;
        5)
            echo "Available profiles:"
            ls "$PROFILE_DIR"/*.prof 2>/dev/null | head -5
            echo -n "Enter profile file path: "
            read profile_file
            generate_flamegraph "$profile_file"
            ;;
        6)
            comprehensive_profile
            ;;
        7)
            clean_profiles
            ;;
        8)
            list_profiles
            ;;
        9)
            echo "Exiting..."
            exit 0
            ;;
        *)
            echo -e "${RED}Invalid option${NC}"
            ;;
    esac
done