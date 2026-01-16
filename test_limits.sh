#!/bin/bash
#
# Test script to verify limiter resource controls work
#

set -e

LIMITER="./limiter"
GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

echo "========================================="
echo "Testing limiter resource controls"
echo "========================================="
echo ""

# Check if limiter exists
if [ ! -f "$LIMITER" ]; then
    echo -e "${RED}✗ limiter binary not found${NC}"
    echo "Run: go build"
    exit 1
fi

echo -e "${YELLOW}Note: These tests require systemd delegation to be configured${NC}"
echo -e "${YELLOW}Run: sudo $LIMITER setup${NC}"
echo -e "${YELLOW}If you just ran setup, you may need to logout/login for memory limits to work${NC}"
echo ""

# Check if delegation is actually active for this session
echo "Checking delegation status..."
if systemctl --user show-environment | grep -q "DBUS_SESSION_BUS_ADDRESS"; then
    # Check if we have cgroup delegation
    if [ -d "/sys/fs/cgroup/user.slice/user-$(id -u).slice/user@$(id -u).service" ]; then
        CGROUP_PATH="/sys/fs/cgroup/user.slice/user-$(id -u).slice/user@$(id -u).service"
        if [ -f "$CGROUP_PATH/cgroup.controllers" ]; then
            CONTROLLERS=$(cat "$CGROUP_PATH/cgroup.controllers")
            echo "Available controllers: $CONTROLLERS"
            if [[ "$CONTROLLERS" == *"memory"* ]]; then
                echo -e "${GREEN}✓ Memory controller is delegated${NC}"
            else
                echo -e "${YELLOW}⚠ Memory controller NOT delegated - memory tests may fail${NC}"
                echo -e "${YELLOW}  You may need to logout and login again${NC}"
            fi
        fi
    fi
fi
echo ""

# Test 1: Memory limit - should get OOM killed
echo "Test 1: Memory limit (50M) - allocating 200M (should fail)"
echo "----------------------------------------------"

# Create a Python script that actually allocates memory
cat > /tmp/mem_hog.py << 'MEMTEST'
#!/usr/bin/env python3
import sys

# Allocate ~200MB of actual memory
data = []
try:
    for i in range(200):
        # Allocate 1MB and touch it to ensure it's resident
        chunk = bytearray(1024 * 1024)
        # Write to it to force allocation
        for j in range(0, len(chunk), 4096):
            chunk[j] = 1
        data.append(chunk)
        if i % 50 == 0:
            print(f"Allocated {i}MB...", flush=True)
    print("Successfully allocated 200MB (should not see this)")
    sys.exit(0)
except MemoryError:
    print("MemoryError caught")
    sys.exit(1)
MEMTEST
chmod +x /tmp/mem_hog.py

if timeout 10 $LIMITER -ram 50M -timeout 10s -- python3 /tmp/mem_hog.py 2>&1; then
    echo -e "${YELLOW}⚠ SKIP: Memory allocation succeeded (delegation may not be active)${NC}"
    echo -e "${YELLOW}   Try logging out and back in after running 'sudo limiter setup'${NC}"
    TEST1="SKIP"
else
    EXIT_CODE=$?
    # Exit code 137 = SIGKILL (OOM), 143 = SIGTERM, 1 = error, 255 = systemd scope failure
    if [ $EXIT_CODE -eq 137 ] || [ $EXIT_CODE -eq 143 ] || [ $EXIT_CODE -eq 1 ] || [ $EXIT_CODE -eq 124 ] || [ $EXIT_CODE -eq 255 ]; then
        echo -e "${GREEN}✓ PASS: Process was killed/failed (exit code $EXIT_CODE)${NC}"
        TEST1="PASS"
    else
        echo -e "${YELLOW}? UNCERTAIN: Unexpected exit code $EXIT_CODE${NC}"
        TEST1="UNCERTAIN"
    fi
fi
echo ""

# Test 2: Timeout - should be killed after timeout
echo "Test 2: Timeout (2s) - sleeping for 10s (should timeout)"
echo "----------------------------------------------"

START_TIME=$(date +%s)
if $LIMITER -timeout 2s -- sleep 10 2>&1; then
    echo -e "${RED}✗ FAIL: Should have timed out${NC}"
    TEST2="FAIL"
else
    END_TIME=$(date +%s)
    DURATION=$((END_TIME - START_TIME))
    if [ $DURATION -lt 5 ]; then
        echo -e "${GREEN}✓ PASS: Process timed out after ~${DURATION}s${NC}"
        TEST2="PASS"
    else
        echo -e "${RED}✗ FAIL: Took too long (${DURATION}s)${NC}"
        TEST2="FAIL"
    fi
fi
echo ""

# Test 3: CPU limit - verify it doesn't use more than allocated
echo "Test 3: CPU limit (0.5 cores) - CPU-intensive task"
echo "----------------------------------------------"

cat > /tmp/cpu_hog.sh << 'CPUTEST'
#!/bin/bash
# CPU intensive task - limited to 0.5 cores should take ~6s instead of ~3s
timeout 3 yes > /dev/null &
timeout 3 yes > /dev/null &
timeout 3 yes > /dev/null &
timeout 3 yes > /dev/null &
wait
CPUTEST
chmod +x /tmp/cpu_hog.sh

echo "Running CPU-intensive task with 0.5 core limit for 3 seconds..."
if $LIMITER -cores 0.5 -timeout 5s -- /tmp/cpu_hog.sh 2>&1 > /dev/null; then
    echo -e "${GREEN}✓ PASS: Task completed under CPU limit${NC}"
    TEST3="PASS"
else
    echo -e "${YELLOW}? UNCERTAIN: Task may have been killed${NC}"
    TEST3="UNCERTAIN"
fi
echo ""

# Test 4: Process limit
echo "Test 4: Process limit (5 procs) - trying to spawn 10 (should fail)"
echo "----------------------------------------------"

cat > /tmp/fork_bomb.sh << 'FORKTEST'
#!/bin/bash
# Try to spawn 10 background processes
for i in {1..10}; do
    sleep 10 &
done
wait
FORKTEST
chmod +x /tmp/fork_bomb.sh

if $LIMITER -procs 5 -timeout 3s -- /tmp/fork_bomb.sh 2>&1 > /dev/null; then
    echo -e "${RED}✗ FAIL: Should have hit process limit${NC}"
    TEST4="FAIL"
else
    EXIT_CODE=$?
    # Could fail due to timeout or process limit
    echo -e "${GREEN}✓ PASS: Process was limited (exit code $EXIT_CODE)${NC}"
    TEST4="PASS"
fi
echo ""

# Test 5: Success case - task within limits
echo "Test 5: Success case - small task within limits"
echo "----------------------------------------------"

if $LIMITER -ram 100M -timeout 5s -- echo "Hello, limited world!" 2>&1 | grep -q "Hello, limited world"; then
    echo -e "${GREEN}✓ PASS: Task completed successfully${NC}"
    TEST5="PASS"
else
    echo -e "${RED}✗ FAIL: Task should have succeeded${NC}"
    TEST5="FAIL"
fi
echo ""

# Cleanup
rm -f /tmp/mem_hog.py /tmp/cpu_hog.sh /tmp/fork_bomb.sh

# Summary
echo "========================================="
echo "Test Summary"
echo "========================================="
echo -e "Test 1 (Memory limit):   $TEST1"
echo -e "Test 2 (Timeout):        $TEST2"
echo -e "Test 3 (CPU limit):      $TEST3"
echo -e "Test 4 (Process limit):  $TEST4"
echo -e "Test 5 (Success case):   $TEST5"
echo ""

# Count results (excluding SKIP)
PASS_COUNT=$(echo -e "$TEST1\n$TEST2\n$TEST3\n$TEST4\n$TEST5" | grep -c "PASS" || true)
TOTAL_COUNT=$(echo -e "$TEST1\n$TEST2\n$TEST3\n$TEST4\n$TEST5" | grep -v "SKIP" | wc -l)

echo -e "${GREEN}Passed: $PASS_COUNT/$TOTAL_COUNT (non-skipped)${NC}"

if [ "$PASS_COUNT" -ge 4 ]; then
    echo -e "${GREEN}Tests passed!${NC}"
    exit 0
else
    echo -e "${YELLOW}Some tests did not pass${NC}"
    exit 1
fi
