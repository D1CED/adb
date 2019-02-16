# A Golang library for interacting with the Android Debug Bridge (adb).

A note from the fork's autor:

*This is a side project of mine. Do not expect any support or replies to
feature or bug request.*

The reason for this fork was that I wasn't happy with the internals.
There were simply too many types.

I flattend most structs, integrated the for what evere reason internal
error package and the wire package. Now functions found in the io.go file
contain some of this functionality. Error handling was in a lot of places
overkill so I replaced all of it simply with github.com/pkg/errors.
Return parameters were used in some places, I removed them for the most
part. In a lot of places strings or []bytes where used when io.Readers
would made more sense. I made use of io.Copy in some places one of the
most important functions in Go IMO.

The only thing I'm currently sorry about is that I ripped all tests out.
The package was designed with a TDD approach, at least I think so and
use of small unittest was common. I table driven tests are more idiomatic
for Go so I the next thing to do is to add them back.

If you are looking for a propper adb client library in Go, take a look
a the implementation privided by Google.
