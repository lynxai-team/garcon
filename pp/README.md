
## Average latency (ns/op)

`go test -test.fullpath=true -benchmem -bench ^BenchmarkParsers$ .`

- Arithmetic mean
- Geometric mean
- Harmonic mean
- Median

Implementation            | samples | Arithmetic | Geometric | Harmonic | Median
--------------------------|--------:|-----------:|----------:|---------:|-------:
`parse3digits`            |       1 |       5.47 |      5.47 |     5.47 |   5.47
`parse6digits`            |       1 |       5.48 |      5.48 |     5.48 |   5.48
`parse5digits`            |       1 |       5.49 |      5.49 |     5.49 |   5.49
`parse1digit`             |       1 |       5.51 |      5.51 |     5.51 |   5.51
`parse2digits`            |       1 |       5.62 |      5.62 |     5.62 |   5.62
`parse7digits`            |       1 |       5.67 |      5.67 |     5.67 |   5.67
`parse8digits`            |       1 |       5.69 |      5.69 |     5.69 |   5.69
`parse4digits`            |       1 |       5.77 |      5.77 |     5.77 |   5.77
`parse9digits`            |       1 |       5.82 |      5.82 |     5.82 |   5.82
`parse10digits`           |       1 |       6.27 |      6.27 |     6.27 |   6.27
`parse11digits`           |       1 |       7.10 |      7.10 |     7.10 |   7.10
`parseDigits`             |      19 |       7.47 |      7.15 |     6.89 |   6.33
`parseDigitsBCE`          |      19 |       7.51 |      7.19 |     6.92 |   6.22
`parseDigitsSwitch`       |      19 |       8.27 |      7.81 |     7.42 |   7.28
`parseDigitsInline`       |      19 |       8.31 |      7.84 |     7.45 |   6.79
`parse12digits`           |       1 |       7.88 |      7.88 |     7.88 |   7.88
`parseDigitsSelect`       |      19 |       8.33 |      7.90 |     7.53 |   7.89
`parseDigitsSelectUnsafe` |      19 |       8.32 |      7.90 |     7.54 |   7.80
`parse13digits`           |       1 |       7.93 |      7.93 |     7.93 |   7.93
`parseDigitsFallthrough`  |      19 |       8.99 |      8.48 |     8.01 |   8.24
`parseDigitsOnly`         |      19 |       9.21 |      8.61 |     8.08 |   8.44
`parse14digits`           |       1 |       9.08 |      9.08 |     9.08 |   9.08
`parseUnsigned`           |      19 |      10.12 |      9.42 |     8.75 |  10.23
`parse15digits`           |       1 |      10.38 |     10.38 |    10.38 |  10.38
`parse17digits`           |       1 |      11.14 |     11.14 |    11.14 |  11.14
`parse16digits`           |       1 |      11.15 |     11.15 |    11.15 |  11.15
`parse18digits`           |       1 |      11.51 |     11.51 |    11.51 |  11.51
`strconvParseUint`        |      19 |      24.50 |     22.04 |    20.34 |  22.09

