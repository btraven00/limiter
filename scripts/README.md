# Test Scripts

Manual testing scripts for limiter resource controls.

## mem_test.py

Memory allocation test that forces actual (resident) memory allocation by writing to every page.

**Usage:**
```bash
# Allocate default 200MB
python3 mem_test.py

# Allocate specific amount
python3 mem_test.py 500

# Test with limiter (should get OOM killed)
../limiter -ram 50M -timeout 10s -- python3 mem_test.py 200
```

**How it works:**
- Allocates memory in 1MB chunks
- Writes to every 4KB page to ensure actual allocation (not just virtual)
- Reports progress every 10MB
- Returns exit code 0 on success, 1 on MemoryError

**Expected behavior with limiter:**
When memory limit is set below the allocation size, the process should be killed by the OOM killer with the message:
```
Process killed: Out of memory (exceeded memory limit)
```

## Notes

- These scripts are for manual testing and debugging
- The main test suite is in `test_limits.sh` at the project root
- Memory allocation tests require Python 3