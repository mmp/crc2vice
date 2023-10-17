`crc2vice` is a utility for people doing facility engineering for
[vice](https://pharr.org/vice). It extracts video maps from installed CRC
ARTCCs and converts them to _vice_'s JSON format.

Usage:

* Open a command prompt and go to your `%AppData%\Local\CRC` directory.
* Run `crc2vice` and give it the name of an installed ARTCC (e.g.,
  `crc2vice ZNY`. You can see which ARTCCs are installed by examining the
  `ARTCCs` folder there.
* Two files will be written, one containing the video map definitions
  (e.g., `ZNY-videomaps.json`) and one with a information about the
  individual maps (e.g., `ZNY.info`). The video maps file can be used
  directly with _vice_. Entries from the information file contain the
  information needed to include a map in the "stars\_maps" section of
  a _vice_ scenario definition.
