# The go-b2bua is a GO port of the [Sippy B2BUA](https://github.com/sippy/b2bua)
## The main differences from the Python B2BUA are:

- Smaller memory foot print (approx 2.5x).
- All available CPU cores are utilized.
- Runs faster (approx 4x per one CPU core).
- The configuration object is not global thus allowing to run several B2BUA instances inside one executable.

## Known problems:

- The standard GO library called net does not support the SO\_REUSEPORT thus you may experience crashes on b2bua\_simple running on default listen address (i.e. 0.0.0.0).
- Only basic SIP stack is available at the moment. No rtpproxy, no CLI interface, no RADIUS.
