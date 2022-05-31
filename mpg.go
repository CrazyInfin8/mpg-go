package mpg

//go:generate go run github.com/gotranspile/cxgo/cmd/cxgo

import (
	"bytes"
	"image"
	"image/color"
	"image/draw"
	"os"
	"time"
	"unsafe"

	"github.com/gotranspile/cxgo/runtime/stdio"
)

// ExpectedHeader is returned by "NewPlayer" functions when a file could not be processed
type ExpectedHeader struct{}

func (err ExpectedHeader) Error() string { return "Unable to find header" }

// Player processes and decodes video and audio in MPG format (MPEG1 video
// encoding and MP2 audio encoding)
type Player struct {
	plm *plm_t

	frame       frame
	hasNewFrame bool

	audioBuffer     *bytes.Buffer
	byteDepth       int
	maxSampleFrames int
}

func newPlayer(p *plm_t) (*Player, error) {
	if plm_has_headers(p) != _true {
		plm_destroy(p)
		return nil, ExpectedHeader{}
	}
	plm := new(Player)
	plm.plm = p
	plm.audioBuffer = new(bytes.Buffer)
	plm.byteDepth = 2
	plm.SetAudioLeadTime(45 * time.Millisecond)
	plm_set_video_decode_callback(plm.plm, videoCallback, unsafe.Pointer(plm))
	plm_set_audio_decode_callback(plm.plm, audioCallback, unsafe.Pointer(plm))
	return plm, nil
}

// NewPlayerFromFile creates a new player from a given file. The file is not
// closed when the player is closed.
func NewPlayerFromFile(f *os.File) (*Player, error) {
	p := plm_create_with_file(stdio.OpenFrom(f), _false)
	return newPlayer(p)
}

// NewPlayerFromFile creates a new player from a given filename.
func NewPlayerFromFilename(file string) (*Player, error) {
	f, err := os.Open(file)
	if err != nil {
		return nil, err
	}
	p := plm_create_with_file(stdio.OpenFrom(f), _true)
	return newPlayer(p)
}

// NewPlayerFromBytes creates a new player from a list of raw bytes.
func NewPlayerFromBytes(data []byte) (*Player, error) {
	p := plm_create_with_memory(&data[0], uint64(len(data)), _false)
	return newPlayer(p)
}

// Close closes the internal player and discards data.
func (plm *Player) Close() {
	plm.frame = frame{}
	plm.audioBuffer.Reset()
	plm_destroy(plm.plm)
	plm.plm = nil
}

// *** Video ***

func videoCallback(p *plm_t, f *plm_frame_t, u unsafe.Pointer) {
	plm := (*Player)(u)
	plm.frame = frame{f}
	plm.hasNewFrame = true
}

// HasVideo returns true if this file contains video.
func (plm *Player) HasVideo() bool { return plm_get_num_video_streams(plm.plm) > 0 }

// NumVideoStreams returns the number of video channels present in the file.
// (0-1)
func (plm *Player) NumVideoStreams() int { return int(plm_get_num_video_streams(plm.plm)) }

// SetVideoEnabled sets whether video should be decododed.
func (plm *Player) SetVideoEnabled(enabled bool) { plm_set_video_enabled(plm.plm, boolToInt(enabled)) }

// VideoEnabled returns true if decoding video is enabled.
func (plm *Player) VideoEnabled() bool { return plm_get_video_enabled(plm.plm) == _true }

// HasNewFrame returns true if a new frame has been decoded. Drawing the current
// frame sets this to false
func (plm *Player) HasNewFrame() bool { return plm.hasNewFrame }

// FrameRate is the number of frames in a second.
func (plm *Player) FrameRate() float64 { return plm_get_framerate(plm.plm) }

// Width is the width of the video
func (plm *Player) Width() int { return int(plm_get_width(plm.plm)) }

// Height is the height of the video.
func (plm *Player) Height() int { return int(plm_get_height(plm.plm)) }

// *** Audio ***

func audioCallback(p *plm_t, samples *plm_samples_t, u unsafe.Pointer) {
	plm := (*Player)(u)
	if l, max := plm.audioBuffer.Len(), plm.maxSampleFrames*plm.byteDepth; max > 0 && l > max*4 {
		l -= max
		var discard [16]byte
		for l > 16 {
			plm.audioBuffer.Read(discard[:])
			l -= 16
		}
		plm.audioBuffer.Read(discard[:l])
	}
	plm.audioBuffer.Grow(plm_audio_samples_per_frame * plm.byteDepth * 2)
	switch plm.byteDepth {
	case 1:
		for i := 0; i < len(samples.Interleaved); i++ {
			b := int8(samples.Interleaved[i] * 0x7F)
			plm.audioBuffer.WriteByte(byte(b))
		}
	case 2:
		for i := 0; i < len(samples.Interleaved); i++ {
			b := int16(samples.Interleaved[i] * 0x7FFF)
			plm.audioBuffer.WriteByte(byte(b))
			plm.audioBuffer.WriteByte(byte(b >> 8))
		}
	case 4:
		for i := 0; i < len(samples.Interleaved); i++ {
			b := int32(samples.Interleaved[i] * 0x7FFFFFFF)
			plm.audioBuffer.WriteByte(byte(b))
			plm.audioBuffer.WriteByte(byte(b >> 8))
			plm.audioBuffer.WriteByte(byte(b >> 16))
			plm.audioBuffer.WriteByte(byte(b >> 24))
		}
	}
}

// HasAudio returns true if this file contains audio.
func (plm *Player) HasAudio() bool { return plm_get_num_audio_streams(plm.plm) > 0 }

// NumAudioStreams returns the number of audio channels that are present in the
// file. (0-4)
func (plm *Player) NumAudioStreams() int { return int(plm_get_num_audio_streams(plm.plm)) }

// SetAudioEnabled sets whether audio should be decoded.
func (plm *Player) SetAudioEnabled(enabled bool) { plm_set_audio_enabled(plm.plm, boolToInt(enabled)) }

//AudioEnabled returns true if decoding audio is enabled.
func (plm *Player) AudioEnabled() bool { return plm_get_audio_enabled(plm.plm) == _true }

// HasNewAudio returns true if the audio buffer still contains unread data.
func (plm *Player) HasNewAudio() bool { return plm.audioBuffer.Len() > 0 }

// ClearAudioBuffer clears the audio buffer.
func (plm *Player) ClearAudioBuffer() { plm.audioBuffer.Reset() }

// SampleRate is how many samples per second in the audio stream.
func (plm *Player) SampleRate() int { return int(plm_get_samplerate(plm.plm)) }

// SetAudioLeadTime sets how long the audio is decoded in advance or behind the
// video decode time. this is typically set to the duration of the buffer of
// your audio API.
func (plm *Player) SetAudioLeadTime(time time.Duration) {
	plm.maxSampleFrames = int(time.Seconds() * float64(plm.SampleRate()) * 1.2)
	plm_set_audio_lead_time(plm.plm, time.Seconds())
}

// AudioLeadTime is how long the audio is decoded in advance or behind the video
// decode time.
func (plm *Player) AudioLeadTime() time.Duration {
	return floatToSecs(plm_get_audio_lead_time(plm.plm))
}

// ByteDepth is how many bytes are in each.
func (plm *Player) ByteDepth() int { return plm.byteDepth }

// SetByteDepth sets how many bytes per sample audio is decoded as. Currently
// the values 1 (8-bit), 2 (16-bit), and 4 (32-bit) are supported.
func (plm *Player) SetByteDepth(depth int) {
	if depth == 1 || depth == 2 || depth == 4 {
		plm.audioBuffer.Reset()
		plm.byteDepth = depth
	}
}

// *** Both ***

// Time is how far the video has progressed.
func (plm *Player) Time() time.Duration { return floatToSecs(plm_get_time(plm.plm)) }

// Duration is how long the entire video is.
func (plm *Player) Duration() time.Duration { return floatToSecs(plm_get_duration(plm.plm)) }

// Rewind moves to the beginning.
func (plm *Player) Rewind() { plm_rewind(plm.plm) }

// Loop returns true if the video was set to loop when finished.
func (plm *Player) Loop() bool { return plm_get_loop(plm.plm) == _true }

// SetLoop sets whether the video should loop when finished.
func (plm *Player) SetLoop(set bool) { plm_set_loop(plm.plm, boolToInt(set)) }

// Finished returns true when the video has ended. This is always false when
// looping.
func (plm *Player) Finished() bool { return plm_has_ended(plm.plm) == _true }

// Decode processes the video accordingly to the duration "elapsed".
func (plm *Player) Decode(elapsed time.Duration) { plm_decode(plm.plm, elapsed.Seconds()) }

// Seek to the specified time.
//
// If "exact" is false, this will seek to the nearest intra frame.
// If "exact" is true, this will seek to the exact time. this can be slower
// as each frame since the last intra frame would need to be decoded.
//
// Seek returns true when successful.
func (plm *Player) Seek(time time.Duration, exact bool) bool {
	return plm_seek(plm.plm, time.Seconds(), boolToInt(exact)) == _true
}

// Read consumes and reads data from the audio buffer. If the audio buffer does
// not have any audio, it sends 0 to buf until it is full.
//
// This is intended to make it easy to create an Ebiten or Oto player straight
// from this *Player
func (plm *Player) Read(buf []byte) (n int, err error) {
	n, _ = plm.audioBuffer.Read(buf)
	if n == 0 {
		n = len(buf)
		for i := 0; i < n; i++ {
			buf[i] = 0x00
		}
	}
	return
}

// *** frame ***

// frame makes it easier to use and draw "plm_frame_t" as an "image.Image"
type frame struct {
	*plm_frame_t
}

func (frame frame) ColorModel() color.Model { return color.YCbCrModel }

func (frame frame) Bounds() image.Rectangle {
	return image.Rectangle{
		image.Point{},
		image.Point{int(frame.plm_frame_t.Width), int(frame.plm_frame_t.Height)},
	}
}

var ycbcrBlack = color.YCbCrModel.Convert(color.Black)

func (frame frame) At(x, y int) color.Color {
	width, height := int(frame.plm_frame_t.Width), int(frame.plm_frame_t.Height)
	if x+y*width < width*height {
		yIndex := x + y*width
		cIndex := x/2 + (y/2)*(width/2)
		return color.YCbCr{
			Y:  index(frame.Y.Data, yIndex),
			Cr: index(frame.Cr.Data, cIndex),
			Cb: index(frame.Cb.Data, cIndex),
		}
	}
	return ycbcrBlack
}

// DrawTo draws the current frame to the image in "img".
func (plm *Player) DrawTo(img draw.Image) {
	if plm.frame.plm_frame_t != nil {
		draw.Draw(img, plm.frame.Bounds(), plm.frame, image.Point{}, draw.Src)
	}
	plm.hasNewFrame = false
}

// ReadRGBA overwrites the passed byte array in "data" in RGBA format.
// Intended for writing the current frame directly to "image.RGBA.Pix".
//
// Alpha channels remain unchanged.
//
// ReadRGBA panics if the size of data does not match the size of the
// frame (width * height * 4).
func (plm *Player) ReadRGBA(data []byte) {
	if plm.frame.plm_frame_t != nil {
		width, height := plm.Width(), plm.Height()
		if len(data) != width*height*4 {
			panic("data should be the same size as Player")
		}
		plm_frame_to_rgba(plm.frame.plm_frame_t, bytesToUintPtr(data), int64(width)*4)
	}
	plm.hasNewFrame = false
}

// DrawFrameAt seeks to the specified time and draws the current frame to the
// image in "img".
//
// If "exact" is false, this will seek to the nearest intra frame.
// If "exact" is true, this will seek to the exact time. this can be slower as
// each frame since the last intra frame would need to be decoded.
//
// DrawFrameAt returns true when successful.
func (plm *Player) DrawFrameAt(img draw.Image, elapsed time.Duration, exact bool) bool {
	f := plm_seek_frame(plm.plm, elapsed.Seconds(), boolToInt(exact))
	if f == nil {
		return false
	}
	src := frame{f}
	draw.Draw(img, img.Bounds(), src, image.Point{}, draw.Src)
	return true
}

// ReadRGBAAt seeks to the specified time and overwrites the passed
// byte array in "data" in RGBA format. Intended mainly for writing the current
// frame directly to "image.RGBA.Pix".
//
// If "exact" is false, this will seek to the nearest intra frame.
// If "exact" is true, this will seek to the exact time. this can be slower as
// each frame since the last intra frame would need to be decoded.
//
// ReadRGBAAt returns true when successful.
//
// Alpha channels remain unchanged.
//
// ReadRGBAAt panics if the size of data does not match the size of the
// frame (width * height * 4).
func (plm *Player) ReadRGBAAt(data []byte, elapsed time.Duration, exact bool) bool {
	width, height := plm.Width(), plm.Height()
	if len(data) != width*height*4 {
		panic("image.RGBA should be the same size as Player")
	}
	f := plm_seek_frame(plm.plm, elapsed.Seconds(), boolToInt(exact))
	if f == nil {
		return false
	}
	plm_frame_to_rgba(f, bytesToUintPtr(data), int64(width*4))
	return true
}
