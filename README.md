flacrate
========

flacrate overrides the sample rate of a FLAC file.
The use case for this is if you have a file that has accidentally been encoded
at the wrong sample rate (so it plays too fast or too slow) and you don't
want to go through the trouble of re-encoding it.
For example, a track that claims to be at 48kHz, but should actually be played at 44.1kHz.

flacrate does not do any resampling or time-stretching. It just sets the sample rate in the metadata.

Limitations
-----

- flacrate can only be used to change a "common" rate to another "common" rate.
  The common rates are 8kHz, 16kHz, 22.05kHz, 24kHz, 32kHz, 44.1kHz, 48kHz, 88.2kHz,
  96kHz, 176.4kHz, and 192kHz.

  The reason is that these rates have a special encoding in the FRAME headers, so going
  from a common rate to a non-common rate or vice versa would involve changing the size
  of each frame.

- flacrate operates on files in-place, using mmap. There is no dry run mode and there is
  no way to undo changes. If flacrate encounters an error in the middle of operation,
  it will stop and print an error, leaving half of your file in a broken, partially-modified
  state.

  Make sure you have backups. 

  (In a pinch, running `flacrate -rate $originalrate` seems to work pretty
  well to recover after an error, but no guarantees.)

- flacrate relies heavily on CRCs in the FLAC file to figure out the boundaries of audio frames.
  It will only work on well-formed files. It will not work if your file is damaged.
  Run `flac --test` before attempting to use flacrate.



Usage
----

Install:

    go get github.com/magical/flacrate

Run:
    
    flacrate -rate 44100 track.flac
