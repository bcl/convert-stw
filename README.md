convert-stw is a simple program to convert Atari STWriter documents into text files.

It recognizes all of the control codes documented in [the
manual](https://inverseatascii.files.wordpress.com/2015/10/s2e41-prt-stwritermanual.pdf).
Thanks to [Inverse ATASCII](https://inverseatascii.info/2016/04/01/s2e41-atari-st-writer/) for
posting it.

To build it you need to have Go installed. Run `make` and it will build the binary.

Convert a document by running `convert-stw --input <stwriter.doc> --output output.txt` or if you leave off
input or output it will use stdin/stdout respectively.
