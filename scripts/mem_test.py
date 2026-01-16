#!/usr/bin/env python3
"""
Memory allocation test script for limiter

This script attempts to allocate a specified amount of memory to test
memory limits. It forces actual memory allocation (not just virtual)
by writing to the allocated bytes.

Usage:
    python3 mem_test.py [SIZE_IN_MB]

Example:
    # Allocate 200MB (default)
    python3 mem_test.py
    
    # Allocate 500MB
    python3 mem_test.py 500
    
Test with limiter:
    limiter -ram 50M -timeout 10s -- python3 scripts/mem_test.py 200
    # Should get OOM killed when trying to allocate 200MB with 50M limit
"""

import sys

def allocate_memory(size_mb):
    """Allocate and touch memory to ensure it's resident"""
    print(f"Attempting to allocate {size_mb}MB of memory...")
    print("(Each chunk is written to ensure actual allocation)")
    
    data = []
    try:
        for i in range(size_mb):
            # Allocate 1MB chunk
            chunk = bytearray(1024 * 1024)
            
            # Write to every page (4KB) to force actual allocation
            # This prevents lazy allocation where memory is only reserved
            for j in range(0, len(chunk), 4096):
                chunk[j] = 1
            
            data.append(chunk)
            
            # Progress reporting
            if i % 10 == 0:
                print(f"Allocated {i}MB...", flush=True)
        
        print(f"\n✓ Successfully allocated {size_mb}MB")
        print(f"  Total data chunks: {len(data)}")
        return 0
        
    except MemoryError as e:
        print(f"\n✗ MemoryError after allocating ~{len(data)}MB")
        print(f"  Error: {e}")
        return 1
    except KeyboardInterrupt:
        print(f"\n⚠ Interrupted after allocating {len(data)}MB")
        return 130

if __name__ == "__main__":
    # Default to 200MB if no argument provided
    size_mb = 200
    
    if len(sys.argv) > 1:
        try:
            size_mb = int(sys.argv[1])
            if size_mb <= 0:
                print("Error: Size must be positive", file=sys.stderr)
                sys.exit(1)
        except ValueError:
            print(f"Error: Invalid size '{sys.argv[1]}' (must be integer)", file=sys.stderr)
            sys.exit(1)
    
    sys.exit(allocate_memory(size_mb))