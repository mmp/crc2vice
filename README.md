`crc2vice` is a utility for people doing facility engineering for
[vice](https://pharr.org/vice). It extracts video maps from installed CRC
ARTCCs and converts them to _vice_'s internal format.

Usage:

* Open a command prompt and go to your `%AppData%\Local\CRC` directory.
* Run `crc2vice` and give it the name of an installed ARTCC (e.g.,
  `crc2vice ZNY`. You can see which ARTCCs are installed by examining the
  `ARTCCs` folder there.
* Two files will be written, one containing the video map details
  (e.g., `ZNY-videomaps.gob`) and one with a information about the
  individual maps (e.g., `ZNY-manifest.gob`). You can then either copy
  both files to the `resources/videomaps` folder in your `%AppData/Local/Vice`
  directory, or can use the `-videomap` command-line option to _vice_;
  if you use `-videomap`, only provide it with the path to the
  `ZXX-videomaps.gob` file. It will look for the `ZXX-manifest.gob` file
  in the same folder.
