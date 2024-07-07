# kevmo314/rtp-to-fmp4

Demonstration of how to convert RTP packets to fmp4 without ffmpeg/gstreamer.

## Usage

Run the application with

```sh
go run main.go
```

Produce RTP packets with

```sh
ffmpeg -loglevel debug -re -f lavfi -i testsrc -pix_fmt yuv420p -bf 0 -g 60 -c:v libx264 -f rtp udp://localhost:5004
```

When fragments are produced, play with

```sh
ffplay -loglevel debug output-00000.mp4
```
