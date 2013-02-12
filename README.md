imgpick
=======

Go package to finds the primary image featured on a webpage

One method "PickImage" is provided which fetches the supplied URL and looks for images on the page.

Inspired by the Reddit scraper: https://github.com/reddit/reddit/blob/a6a4da72a1a0f44e0174b2ad0a865b9f68d3c1cd/r2/r2/lib/scraper.py#L57-84

Run the sample command line app like this:

go run /path/to/bin/picker.go  http://example.com/ /path/to/output/img.png

INSTALLATION
============

Simply run

	go get github.com/iand/imgpick

Documentation is at [http://go.pkgdoc.org/github.com/iand/imgpick](http://go.pkgdoc.org/github.com/iand/imgpick)

LICENSE
=======
This code and associated documentation is in the public domain.

To the extent possible under law, Ian Davis has waived all copyright
and related or neighboring rights to this file. This work is published 
from the United Kingdom. 