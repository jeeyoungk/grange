Grange
======

Grange implements a modern subset of the range query language. It is an
expressive grammar for selecting information out of arbitrary, self-referential
metadata. It was developed for querying information about hosts across
datacenters.

    %{has(DC;east) & has(TYPE;redis)}:DOWN

See [godocs](https://godoc.org/github.com/xaviershay/grange) for usage and
syntax.

Goals
-----

* Easily run cross-platform.
* Error messages when things go wrong.
* Fast. (Looking at you, `clusters`.)

Development
-----------

This is library, so does not export a main function. Run it via tests.

    Check out github.com/xaviershay/range-spec and set env var RANGE_SPEC_PATH
    to its location.

    go get github.com/xaviershay/peg

    $GOPATH/bin/peg range.peg && go test
