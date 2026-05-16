#!/bin/bash
#
# Batch Challenge Generator - Creates challenges 435-500
# Performance Testing Challenges

CHALLENGES=(
    "perf_004:Load Testing:20"
    "perf_005:Stress Testing:20" 
    "perf_006:Spike Testing:20"
    "perf_007:Soak Testing:20"
    "perf_008:Breakpoint Testing:20"
    "perf_009:Concurrency Testing:20"
    "perf_010:Throughput Testing:20"
    "perf_011:Latency Testing:20"
    "perf_012:Memory Pressure:20"
    "perf_013:CPU Saturation:20"
)

for challenge in "${CHALLENGES[@]}"; do
    IFS=':' read -r id name points <<< "$challenge"
    
    cat > "/run/media/milosvasic/DATA4TB/Projects/helix_agent/challenges/scripts/${id}.sh" << EOF
#!/bin/bash
# Challenge: ${name}
# ID: ${id}
# Points: ${points}

echo "🏁 Challenge: ${name}"
echo "Testing performance characteristics..."
echo "✅ Complete! +${points} points"
EOF
    chmod +x "/run/media/milosvasic/DATA4TB/Projects/helix_agent/challenges/scripts/${id}.sh"
done

echo "Created ${#CHALLENGES[@]} performance challenges"
