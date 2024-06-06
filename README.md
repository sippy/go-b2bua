[![Go Build](https://github.com/sippy/go-b2bua/actions/workflows/go.yml/badge.svg)](https://github.com/sippy/go-b2bua/actions/workflows/go.yml)

# The go-b2bua is a GO port of the [Sippy B2BUA](https://github.com/sippy/b2bua)
## The main differences from the Python B2BUA are:

- Smaller memory foot print (approx 2.5x).
- All available CPU cores are utilized.
- Runs faster (approx 4x per one CPU core).
- The configuration object is not global thus allowing to run several B2BUA instances inside one executable.
