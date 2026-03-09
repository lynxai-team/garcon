# ultra-fast ASCII digits to `int` parser

## Average latency (ns/op)

`go test -test.fullpath=true -benchmem -run=^$ -bench ^BenchmarkParsers$ github.com/lynxai-team/garcon/pp`

- Arithmetic mean
- Geometric mean
- Harmonic mean
- Median

Implementation            | # samples | Arithmetic |  Geometric |   Harmonic |     Median
--------------------------|----------:|-----------:|-----------:|-----------:|----------:
`parse4Digits`            |         1 |  **5.595** |      5.595 |      5.595 |      5.595
`parse1Digit`             |         1 |  **5.596** |      5.596 |      5.596 |      5.596
`parse3Digits`            |         1 |  **5.662** |      5.662 |      5.662 |      5.662
`parse2Digits`            |         1 |  **5.709** |      5.709 |      5.709 |      5.709
`parse5Digits`            |         1 |  **5.709** |      5.709 |      5.709 |      5.709
`parse6Digits`            |         1 |  **5.755** |      5.755 |      5.755 |      5.755
`parse7Digits`            |         1 |  **5.807** |      5.807 |      5.807 |      5.807
`parse8Digits`            |         1 |  **5.899** |      5.899 |      5.899 |      5.899
`parse9Digits`            |         1 |  **5.984** |      5.984 |      5.984 |      5.984
`parseDigits`             |        19 |  **7.705** |  **7.370** |  **7.124** |  **6.351**
`parseDigitsSwitch`       |        19 |  **9.113** |  **8.370** |  **7.810** |  **6.314**
`parseDigitsInline`       |        19 |  **9.133** |  **8.390** |  **7.820** |  **6.374**
`parseDigitsOnly`         |        19 |  **9.499** |  **8.890** |  **8.360** |  **8.523**
`parseDigitsFallthrough`  |        19 |  **9.538** |  **8.860** |  **8.260** |  **8.351**
`parseDigitsSelect`       |        19 |  **9.781** |  **8.980** |  **8.280** |  **8.042**
`parseDigitsSelectUnsafe` |        19 |  **9.866** |  **9.010** |  **8.300** |  **8.003**
`parseUnsigned`           |        19 | **10.492** |  **9.700** |  **9.000** | **10.720**
`strconv.ParseUint` (std) |        19 | **22.954** | **20.680** | **19.070** | **20.750**
