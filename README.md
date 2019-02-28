# A Golang library for interacting with the Android Debug Bridge (adb).

A note from the fork's autor:

*This is a side project of mine. Do not expect any support or replies to
feature or bug request.*

The reason for this fork was that I wasn't happy with the internals.
There were simply too many types and it seemd overcomplex.

<!-- I flattend most structs, integrated the for what ever reason internal
error package and the wire package. Now functions found in the io.go file
contain some of this functionality. Error handling was in a lot of places
overkill so I replaced all of it simply with github.com/pkg/errors.
Return parameters were used in some places, I removed them for the most
part. In a lot of places strings or []bytes where used when io.Readers
would made more sense. I made use of io.Copy in some places one of the
most important functions in Go IMO.

A quick note about finalizers and goroutines:
If you start a gorutine and a reference to an object lives on its stack
the object will never be deallocated so the finalizer will never be run.
*Again do not use finalizers!*
-->

The only thing I'm currently sorry about is that I ripped out all tests.
The package was designed with a TDD approach, at least I think so and
use of small unittest was common. I favour table driven tests as they are more
idiomatic in Go so the next step is to add them back.

If you are looking for a propper adb client library in Go, take a look
a the implementation provided by Google.

## DONE (mostly):
Asyncwriter rewritten
  - now counts based on bytes written
  - does not filestat (racy?)
removed useage of runtime.SetFinalizer
  - this is considered bad style
  - better use Close properly
flatten
  - remove unessecarry types
  - remove top-level sybols
  - inline functions
  - do things in-place (tetra to le int)
  - get rid of dependencies (rhttp!?)
  - simplify names
simplified error handling (maybe readd some)
  - why internal package?
  - no buissness in this
  - now uses pkg/errors
removed useage of return parameters
moved away from []byte/strings to io.Reader
  - use more io.Copy, io.ReaderTo, io.WriterFrom
removed adb/wire
  - abstraction was broken anyways
  - don't export interfaces
  - moved to net.Conn and io.Reader/Writer
'log' useage of device watcher removed (moved to fmt.Printf, missing timestamp)
reimplement command handling
  - what about `echo :$?` (this is actually fine)
  - look into documentation for a better way to get exit status
rework how device descriptor work and how device connects to the server
  - always use serial as query-prefix `host-serial: &lt serial-number &gt : request
  - better track connection state (when reuse, when drop)
socket read/writes
  - only use two reads per message, header and body
  - only allocate once
  - use io.ReadFull
  - investigate if write full neccessary
  - reduce allocations for message sends
clean-up io
change ForwartSpec to a simple string
implement os.FileInfo for dir_entries
device info remove map allocations
device watcher implement shutdown
device watcher map allocation in loop
move cmd/demo and cmd/raw-adb to example files

## TODO:
think about features of device_extra (what to keep/remove?)
write more tests (table driven style)
track (potential) leakages
fix device watcher 'calculateStateDiffs'
implement io.Closer for deviceWatcher
expose low-level adb interface
implement formatter for deviceInfo

Notice About hierarchy flattening:
See: https://github.com/golang/go/wiki/CodeReviewComments#interfaces

Device (new)
- Server
  - path
  - address
- serial

Device (old)
- sever (interface)
  - realServer
    - ServerConfig
      - PathToADB
      - Host
      - Port
      - Dialer (interface)
        - tcpDialer
        - wire.Conn
          - Scanner (interface)
            - realScanner
              - SyncScanner (interface)
                - realSyncScanner
                - StatusReader
            - StatusReader
          - Sender (interface)
            - realSender
            - SyncSender (interface)
              - realSyncSender
      - filesystem
    - address
- DeviceDescriptor
  - deviceDescriptorType
  - serial
- deviceListFunc

## Status
Host:
  - version [x]
  - kill [x]
  - devices(-l) [x]
  - track-devices [ ] needs investigation
 
Device:
  - get-devpath [x]
  - get-state; check this beforehand, poll?
  - Forward:
      - foreward
      - norebind
      - killforward
      - list-forward
  - shell [x]
  - remount [ ] result type need investigation, use this before sync/push
  - dev [x]
  - tcp?
  - local?
  - framebuffer?
  - jdwp?
  - reverse?

Sync:
  - List [x]
  - Send [ ] change abstraction to accepting reader
  - Retrieve [x]
  - Stat [x]

Other:
  - host: emulator; don't implement this, not for clients
  - device: product N/A
  - device: get-serialno; uneccessary as this is the device ID