# MPG-Go
## Pure Go mpg decoding made from [pl_mpeg]
[![GoDoc](https://godoc.org/github.com/crazyinfin8/mpg-go?status.svg)](https://pkg.go.dev/github.com/crazyinfin8/mpg-go) [![GoReportCard](https://goreportcard.com/badge/github.com/crazyinfin8/mpg-go)](https://goreportcard.com/report/github.com/crazyinfin8/mpg-go)

[Mpg-go] is a pure Go (no CGo) MPG decoder and player. It is made by transpiling [pl_mpeg] from C to Go using the [cxgo] translation tool.

[Mpg-go]'s Goal is to provide an easy to use, pure Go software video and audio decoder. It provides functions meant for drawing frames to `image/draw`'s `Image`s, as well as writing directly to `image`'s `RGBA.Pix`, and provides and easy to use audio reader made to work effortlessly in Ebiten or Oto. For a working example project showing many features of [mpg-go], see [player].

As [pl_mpeg] and [cxgo] are both experimental, [mpg-go] should also be experimental as well.

# Encoding

[Pl_mpeg] supports mpg file container with MPEG1 ("mpeg1") video encoding and MPEG1 Audio Layer II ("mp2") audio encoding. While MPEG1 and MP2 is seen as outdated and inefficient, it's patents should have expired by now making it 

You can encode your own videos to mpg with FFMPEG using the following command:

```bash
ffmpeg -i <YOUR_ORIGINAL_VIDEO> -c:v mpeg1video -c:a mp2 -q:v 0 <YOUR_OUTPUT_VIDEO>.mpg
```

This will convert the video from `<YOUR_ORIGINAL_VIDEO>` to `<YOUR_OUTPUT_VIDEO>.mpg` with the right codec and variable bitrate.

# Getting started

## Running example
To run the example, run this command:

```bash
go install github.com/crazyinfin8/mpg-go/example/player@latest
go run github.com/crazyinfin8/mpg-go/example/player
```

## Install

To install this library for use in your own project, run the following command:

```bash
go get github.com/crazyinfin8/mpg-go
```

## Usage

A new player can be created using one of the following functions

```go
import "github.com/crazyinfin8/mpg-go"

player, err := mpg.NewPlayerFromFile(*io.File)
player, err := mpg.NewPlayerFromFilename(string)
player, err := mpg.NewPlayerFromBytes([]byte)
```

Set up your graphics library

```go
import "github.com/crazyinfin8/mpg-go"
import "image"

if player.HasVideo() {
    width, height := player.Width(), player.Height()
    img := image.NewRGBA(image.Rect(0, 0, width, height))
    
    // if using functions ReadRGBA and ReadRGBAAt,
    // set alpha channels because those functions do not set them.
    mpg.SetAlpha(img.Pix)
}
```

Set up your audio library

```go
import "github.com/hajimehoshi/ebiten/v2/audio"
import "time"

var ctx *audio.Context
var stream *audio.Player

if player.HasAudio() {
    samplerate := player.SampleRate()
    ctx = audio.NewContext(samplerate)
    stream = ctx.NewPlayer(player)
    player.SetByteDepth(2) // Ebiten uses 16-bit audio

    // Using default buffer size
    stream.SetBufferSize(player.AudioLeadTime())

    //setting a custom buffer size
    stream.SetBufferSize(50 * time.Millisecond)
    player.SetAudioLeadTime(50 * time.Millisecond)

    stream.Play()
}
```

Decode video and audio.

```go
import "time"

framerate := player.FrameRate()

// Call this every frame to progress through and decode the video
//
// This moves at a fized rate for each frame, however it might be more smooth to 
// calculate a time delta per frame.
player.Decode(time.Duration(1 / framerate * float64(time.Second)))
```

Display the video (audio using Ebiten's `audio.Player` should already be playing)

```go
if player.HasNewFrame() {
    // Sets a pixel byte array directly. Note that the pixel array should be the
    // same size as the frame i.e. width*height*4.
    player.ReadRGBA(img.Pix)
    // Draws itself to a "draw.Image"
    player.DrawTo(img)

    // Audio is already playing through the Ebiten audio stream.
}
```

Cleanup when finished

```go
if player.Finished() {
    // video playback is done!
}

player.Close()
```

# TODO

- Add functions to just decode all frames and audio.
- Make it easier to use mpg-go in other graphic libraries such as SDL and Raylib.

[pl_mpeg]:https://github.com/phoboslab/pl_mpeg
[mpg-go]:https://pkg.go.dev/github.com/crazyinfin8/mpg-go
[cxgo]:https://github.com/gotranspile/cxgo
[player]:example/player/