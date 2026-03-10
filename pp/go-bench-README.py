#!/usr/bin/env python3
# -*- coding: utf-8 -*-

"""
Run Go benchmarks and update README.md with latency statistics.

This script executes the Go test command, parses the output, calculates
statistical metrics (Mean, Geometric Mean, Harmonic Mean, Median), and
updates a markdown table in the README.md file.
"""

import re
import sys
import subprocess
import statistics
import argparse
from pathlib import Path
from typing import Dict, List, Tuple

# The regex matches the benchmark name (e.g., BenchmarkParsers/parseDigitsSwitch-0-digits-24)
# It extracts the function name (parseDigitsSwitch) and the latency value.
BENCH_LINE_RE = re.compile(
    r"BenchmarkParsers/(?P<func>[^-]+)-\d+-digits-\d+.*?(?P<val>\d+\.\d+)\s+ns/op"
)

def parse_benchmark_output(output: str) -> Dict[str, List[float]]:
    """
    Parses the Go benchmark output and returns a dictionary mapping function names to lists of latencies.
    """
    data = {}
    for line in output.splitlines():
        match = BENCH_LINE_RE.search(line)
        if match:
            func = match.group("func")
            val = float(match.group("val"))
            data.setdefault(func, []).append(val)
    return data

def compute_stats(data: Dict[str, List[float]]) -> List[Tuple[str, int, float, float, float, float]]:
    """
    Computes statistics for each function and returns a list of tuples sorted by mean latency.
    """
    rows = []
    for func, values in data.items():
        values.sort()
        n = len(values)
        # Python 3.8+ statistics module has geometric_mean and harmonic_mean
        arith = statistics.mean(values)
        geo = statistics.geometric_mean(values) if n > 1 else values[0]
        harm = statistics.harmonic_mean(values) if n > 1 else values[0]
        median = statistics.median(values)
        rows.append((func, n, arith, geo, harm, median))
    
    # Sort by arithmetic mean (fastest first)
    rows.sort(key=lambda x: x[3])
    return rows

def generate_markdown_table(rows: List[Tuple[str, int, float, float, float, float]], go_cmd: str) -> List[str]:
    """
    Generates the markdown table as a list of strings.
    """
    lines = []
    lines.append("## Average latency (ns/op)")
    lines.append("")
    lines.append(f"`{go_cmd}`")
    lines.append("")
    lines.append("- Arithmetic mean")
    lines.append("- Geometric mean")
    lines.append("- Harmonic mean")
    lines.append("- Median")
    lines.append("")
    # Table header
    lines.append("Implementation            | samples | Arithmetic | Geometric | Harmonic | Median")
    lines.append("--------------------------|--------:|-----------:|----------:|---------:|-------:")
    
    # Format rows
    for func, n, arith, geo, harm, median in rows:
        func = f"`{func}`"
        # Right-align numbers in the table
        lines.append(f"{func:<25} | {n:>7} | {arith:>10.2f} | {geo:>9.2f} | {harm:>8.2f} | {median:>6.2f}")
    
    return lines

def update_readme(readme_path: Path, table_lines: List[str], marker: str = "## Average latency (ns/op)"):
    """
    Updates the README.md file by replacing the table section.
    It finds the marker section and replaces it with the new table.
    """
    try:
        content = readme_path.read_text().splitlines(keepends=False)
    except FileNotFoundError:
        content = []

    # Find the marker and replace everything from there until the next header or EOF
    marker_index = -1
    for i, line in enumerate(content):
        if line.strip() == marker:
            marker_index = i
            break
    
    if marker_index >= 0:
        # Remove old table content (until next header or EOF)
        # We'll slice the list and replace
        end_index = len(content)
        for i in range(marker_index + 1, len(content)):
            if content[i].strip().startswith("## "):
                end_index = i
                break
        # Replace the section
        new_content = content[:marker_index] + table_lines + [""] + content[end_index:]
    else:
        # Append to the end
        new_content = content + [""] + table_lines + [""]

    readme_path.write_text("\n".join(new_content) + "\n")

def main():
    parser = argparse.ArgumentParser(description="Update README.md with Go benchmark statistics.")
    parser.add_argument("package", nargs="*", default=["."], help="Go package path (default: current directory)")
    parser.add_argument("--bench", default="BenchmarkParsers", help="Benchmark name filter")
    parser.add_argument("--readme", default="README.md", help="Path to README.md file")
    parser.add_argument("--go-test-flags", default="-test.fullpath=true -benchmem -bench", help="Go test flags (excluding package and bench name)")
    args = parser.parse_args()

    # Construct Go command
    go_cmd = ["go", "test"]
    # Add flags
    go_cmd.extend(args.go_test_flags.split())
    go_cmd.append(f"^{args.bench}$")
    go_cmd.extend(args.package)

    print(f"Running: {' '.join(go_cmd)}")
    try:
        result = subprocess.run(go_cmd, capture_output=True, text=True, check=True)
        output = result.stdout
    except subprocess.CalledProcessError as e:
        print(f"Go test failed: {e.stderr}")
        sys.exit(1)

    # Parse output
    data = parse_benchmark_output(output)
    if not data:
        print("No benchmark data found.")
        sys.exit(0)

    # Compute stats
    rows = compute_stats(data)

    # Generate table
    table_lines = generate_markdown_table(rows, " ".join(go_cmd))

    # Update README
    readme_path = Path(args.readme)
    update_readme(readme_path, table_lines)
    print(f"Updated {readme_path} with new benchmark table.")

if __name__ == "__main__":
    main()
